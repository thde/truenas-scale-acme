package scale

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	DefaultBaseURL   = "http://localhost/api/v2.0/"
	DefaultUserAgent = "truenas-scale-acme/0.1"
)

// ErrStatus respresents a non success status code error.
var ErrStatus = errors.New("status code error")

type Client struct {
	baseURL   *url.URL
	userAgent string
	apiKey    string

	httpClient *http.Client
}

type Option func(*Client)

// NewClient creates a new API client for TrueNAS Scale.
func NewClient(opts ...Option) *Client {
	u, _ := url.Parse(DefaultBaseURL)

	client := Client{
		baseURL:    u,
		userAgent:  DefaultUserAgent,
		httpClient: &http.Client{},
	}

	for _, opt := range opts {
		opt(&client)
	}

	return &client
}

// WithAPIKey defines the API key to be used.
func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithBaseURL defines the API's base url.
func WithBaseURL(u *url.URL) Option {
	return func(c *Client) {
		c.baseURL = u
	}
}

// WithUserAgent allows to change the user agent.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		c.userAgent = userAgent
	}
}

// WithHTTPClient allows to replace the http client.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		c.httpClient = h
	}
}

func (c *Client) newRequest(
	ctx context.Context,
	method, path string,
	params url.Values,
	body interface{}, //nolint:unparam
) (*http.Request, error) {
	if params == nil {
		params = url.Values{}
	}

	rel := &url.URL{Path: path}
	u := c.baseURL.ResolveReference(rel)
	u.RawQuery = params.Encode()

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)

		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	return req, nil
}

func (c *Client) doJSON(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
		err = json.NewDecoder(resp.Body).Decode(&v)
		if err != nil {
			return nil, err
		}
	}

	return resp, err
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, fmt.Errorf("%s: %d, %w", http.StatusText(resp.StatusCode), resp.StatusCode, ErrStatus)
	}

	return resp, err
}
