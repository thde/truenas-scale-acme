package truenas

import (
	"context"
	"errors"
	"fmt"
	"time"

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
//
// Setting ui_certificate does not apply immediately: the server returns this
// response, waits ui_restart_delay seconds, then restarts the UI (dropping the
// WebSocket connection). The new configuration is rolled back to the previous
// one unless system.general.checkin is called within rollback_timeout. We
// therefore wait for the restart, reconnect to the new UI with backoff, and
// confirm the change before the rollback window closes.
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

	// The update returns before the restart (the server delays it by
	// ui_restart_delay so this response is delivered), so any error here is real.
	start := time.Now()
	if _, err := c.a.SystemGeneralUpdate(ctx, params); err != nil {
		return fmt.Errorf("system.general.update: %w", err)
	}

	// Bound the reconnect and checkin to the rollback window. If we miss it, the
	// server safely reverts to the previous certificate and this returns an error.
	deadline := start.Add(time.Duration(rollbackTimeout) * time.Second)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	// Wait for the UI restart to begin before reconnecting, so we don't connect
	// to the about-to-restart UI only to be dropped again.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(restartDelay) * time.Second):
	}

	if err := c.reconnectWithBackoff(ctx); err != nil {
		return fmt.Errorf("reconnect after ui restart: %w", err)
	}

	// Confirm the change. A transient drop right after reconnect can occur, so
	// reconnect once more and retry on a connection error.
	if err := c.a.SystemGeneralCheckin(ctx); err != nil {
		var connErr *jsonrpc.RPCConnectionError
		if !errors.As(err, &connErr) {
			return fmt.Errorf("system.general.checkin: %w", err)
		}
		if rerr := c.reconnectWithBackoff(ctx); rerr != nil {
			return fmt.Errorf("reconnect before checkin: %w", rerr)
		}
		if err := c.a.SystemGeneralCheckin(ctx); err != nil {
			return fmt.Errorf("system.general.checkin: %w", err)
		}
	}

	return nil
}
