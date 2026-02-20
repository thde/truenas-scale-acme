package truenas

import (
	"context"
	"errors"
	"fmt"

	jsonrpc "github.com/filecoin-project/go-jsonrpc"
)

// SystemGeneralEntry holds the system general configuration.
type SystemGeneralEntry struct {
	ID            int          `json:"id"`
	UICertificate *Certificate `json:"ui_certificate"`
	UIHTTPSPort   int          `json:"ui_httpsport"`
}

// SystemGeneralUpdateParams holds parameters for updating system general config.
type SystemGeneralUpdateParams struct {
	UICertificate   *int `json:"ui_certificate,omitempty"`
	UIRestartDelay  *int `json:"ui_restart_delay,omitempty"`
	RollbackTimeout *int `json:"rollback_timeout,omitempty"`
}

// SystemGeneralConfig returns the system general configuration.
func (c *Client) SystemGeneralConfig(ctx context.Context) (*SystemGeneralEntry, error) {
	var result *SystemGeneralEntry
	err := c.withReconnect(ctx, func() error {
		var err error
		result, err = c.a.SystemGeneralConfig(ctx)
		return err
	})
	return result, err
}

// SystemGeneralUpdate updates the system general configuration and confirms the change.
// It sets the UI certificate and waits for the server to apply it, then calls checkin.
func (c *Client) SystemGeneralUpdate(ctx context.Context, params SystemGeneralUpdateParams) error {
	restartDelay := 2
	if params.UIRestartDelay != nil {
		restartDelay = *params.UIRestartDelay
	} else {
		params.UIRestartDelay = &restartDelay
	}

	rollbackTimeout := 30
	if params.RollbackTimeout != nil {
		rollbackTimeout = *params.RollbackTimeout
	} else {
		params.RollbackTimeout = &rollbackTimeout
	}

	// The update triggers a UI restart; the WebSocket connection will drop.
	// We pass the restart delay and rollback timeout server-side.
	_, err := c.a.SystemGeneralUpdate(ctx, params)
	if err != nil {
		// A connection drop after the update is expected (the server restarts).
		// Ignore RPCConnectionError; propagate all other errors.
		var connErr *jsonrpc.RPCConnectionError
		if !errors.As(err, &connErr) {
			return fmt.Errorf("system.general.update: %w", err)
		}
	}

	// Reconnect after the server restarts.
	c.mu.Lock()
	rerr := c.reconnect(ctx)
	c.mu.Unlock()
	if rerr != nil {
		return fmt.Errorf("reconnect after system general update: %w", rerr)
	}

	// Confirm the change is accepted.
	return c.withReconnect(ctx, func() error {
		return c.a.SystemGeneralCheckin(ctx)
	})
}
