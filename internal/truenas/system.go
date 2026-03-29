package truenas

import "context"

// SystemInfo contains basic information about the TrueNAS system.
type SystemInfo struct {
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
}

// SystemInfo returns basic system information.
func (c *Client) SystemInfo(ctx context.Context) (*SystemInfo, error) {
	var result *SystemInfo
	err := c.withReconnect(ctx, func() error {
		var err error
		result, err = c.a.SystemInfoMethod(ctx)
		return err
	})
	return result, err
}
