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
	CertificateCreate    func(ctx context.Context, params CertificateCreateParams) (*Certificate, error)          `rpc_method:"certificate.create"`
	CertificateDelete    func(ctx context.Context, id int, force bool) (bool, error)                              `rpc_method:"certificate.delete"`
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

// withReconnect runs f, and if it returns an RPCConnectionError, attempts one
// reconnect before returning.
func (c *Client) withReconnect(ctx context.Context, f func() error) error {
	err := f()
	if err == nil {
		return nil
	}
	var connErr *jsonrpc.RPCConnectionError
	if !errors.As(err, &connErr) {
		return err
	}

	c.mu.Lock()
	rerr := c.reconnect(ctx)
	c.mu.Unlock()
	if rerr != nil {
		return fmt.Errorf("reconnect failed: %w (original: %v)", rerr, err)
	}

	return f()
}
