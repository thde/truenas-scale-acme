// Package truenas provides a Go client for the TrueNAS WebSocket JSON-RPC 2.0 API:
// https://api.truenas.com/v25.10/index.html
package truenas

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	jsonrpc "github.com/filecoin-project/go-jsonrpc"
	"github.com/gorilla/websocket"
)

const (
	DefaultURL = "ws://localhost/api/current"
)

type api struct {
	AuthLoginWithAPIKey  func(ctx context.Context, apiKey string) (bool, error)                                   `rpc_method:"auth.login_with_api_key"`
	SystemInfoMethod     func(ctx context.Context) (*SystemInfo, error)                                           `rpc_method:"system.info"`
	CoreGetMethods       func(ctx context.Context) (json.RawMessage, error)                                       `rpc_method:"core.get_methods"`
	CertificateQuery     func(ctx context.Context) ([]Certificate, error)                                         `rpc_method:"certificate.query"`
	CertificateCreate    func(ctx context.Context, params CertificateCreateParams) (int, error)                   `rpc_method:"certificate.create"`
	CertificateDelete    func(ctx context.Context, id int, force bool) (int, error)                               `rpc_method:"certificate.delete"`
	CoreGetJobs          func(ctx context.Context, filters [][]any, options jobQueryOptions) ([]Job, error)       `rpc_method:"core.get_jobs"`
	SystemGeneralConfig  func(ctx context.Context) (*SystemGeneralEntry, error)                                   `rpc_method:"system.general.config"`
	SystemGeneralUpdate  func(ctx context.Context, params SystemGeneralUpdateParams) (*SystemGeneralEntry, error) `rpc_method:"system.general.update"`
	SystemGeneralCheckin func(ctx context.Context) error                                                          `rpc_method:"system.general.checkin"`
}

// Client is a TrueNAS SCALE API client.
type Client struct {
	mu     sync.Mutex
	a      api
	closer jsonrpc.ClientCloser

	apiKey string
	opts   []Option
}

type config struct {
	url       *url.URL
	tlsConfig *tls.Config
}

// Option configures a Client.
type Option func(*config)

// WithURL configures the client to connect to the specified URL.
func WithURL(u *url.URL) Option {
	return func(c *config) {
		c.url = u
	}
}

// WithTLSConfig configures TLS settings for the WebSocket connection.
func WithTLSConfig(tc *tls.Config) Option {
	return func(c *config) {
		c.tlsConfig = tc
	}
}

func dial(ctx context.Context, apiKey string, opts []Option) (api, jsonrpc.ClientCloser, error) {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}

	addr := cfg.url
	if addr == nil {
		u, err := url.Parse(DefaultURL)
		if err != nil {
			return api{}, nil, fmt.Errorf("parse default URL: %w", err)
		}

		addr = u
	}

	if cfg.tlsConfig != nil {
		prev := websocket.DefaultDialer.TLSClientConfig
		websocket.DefaultDialer.TLSClientConfig = cfg.tlsConfig
		defer func() { websocket.DefaultDialer.TLSClientConfig = prev }()
	}

	var a api
	closer, err := jsonrpc.NewMergeClient(
		ctx,
		addr.String(),
		"",
		[]any{&a},
		nil,
		jsonrpc.WithNoReconnect(),
		jsonrpc.WithMethodNameFormatter(jsonrpc.DefaultMethodNameFormatter),
	)
	if err != nil {
		return api{}, nil, fmt.Errorf("jsonrpc connect: %w", err)
	}

	ok, err := a.AuthLoginWithAPIKey(ctx, apiKey)
	if err != nil {
		closer()
		return api{}, nil, fmt.Errorf("auth: %w", err)
	}
	if !ok {
		closer()
		return api{}, nil, errors.New("auth: invalid API key")
	}

	return a, closer, nil
}

// Dial connects to TrueNAS SCALE at host and authenticates with apiKey.
func Dial(ctx context.Context, apiKey string, opts ...Option) (*Client, error) {
	a, closer, err := dial(ctx, apiKey, opts)
	if err != nil {
		return nil, err
	}

	return &Client{
		a:      a,
		closer: closer,
		apiKey: apiKey,
		opts:   opts,
	}, nil
}

// Close closes the underlying connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closer != nil {
		c.closer()
	}
}

// reconnect tears down the old connection and establishes a fresh authenticated one.
// Must be called with c.mu held.
func (c *Client) reconnect(ctx context.Context) error {
	if c.closer != nil {
		c.closer()
	}
	a, closer, err := dial(ctx, c.apiKey, c.opts)
	if err != nil {
		return err
	}
	c.a = a
	c.closer = closer
	return nil
}

// reconnectTimeout bounds how long withReconnect keeps retrying to reconnect
// when the server is unavailable (e.g. while the UI is restarting).
const reconnectTimeout = 60 * time.Second

// withReconnect runs f, and if it fails with a transport-level error (a dropped
// or refused connection), reconnects with backoff and retries f once.
//
// The go-jsonrpc library wraps every client-side/transport failure in
// *jsonrpc.ErrClient (this includes "websocket routine exiting" after the
// connection drops, and dial failures). Server-side RPC errors are returned as
// *jsonrpc.JSONRPCError instead, and must propagate unchanged.
func (c *Client) withReconnect(ctx context.Context, f func() error) error {
	err := f()
	if err == nil {
		return nil
	}
	var clientErr *jsonrpc.ErrClient
	if !errors.As(err, &clientErr) {
		return err
	}

	rctx, cancel := context.WithTimeout(ctx, reconnectTimeout)
	defer cancel()
	if rerr := c.reconnectWithBackoff(rctx); rerr != nil {
		return fmt.Errorf("reconnect failed: %w (original: %v)", rerr, err)
	}

	return f()
}

// reconnectWithBackoff repeatedly attempts to reconnect with exponential
// backoff until it succeeds or ctx is cancelled. It is used when the server is
// known to be briefly unavailable (e.g. during a UI restart) and a single
// reconnect attempt would race the restart.
func (c *Client) reconnectWithBackoff(ctx context.Context) error {
	const (
		minDelay = 200 * time.Millisecond
		maxDelay = 5 * time.Second
	)

	delay := minDelay
	for {
		c.mu.Lock()
		err := c.reconnect(ctx)
		c.mu.Unlock()
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("%w (last reconnect error: %v)", ctx.Err(), err)
		case <-time.After(delay):
		}

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}
