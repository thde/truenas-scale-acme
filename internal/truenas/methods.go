package truenas

import (
	"context"
	"encoding/json"
)

// CoreGetMethods returns the raw JSON introspection data from core.get_methods.
func (c *Client) CoreGetMethods(ctx context.Context) (json.RawMessage, error) {
	var result json.RawMessage
	err := c.withReconnect(ctx, func() error {
		var err error
		result, err = c.a.CoreGetMethods(ctx)
		return err
	})
	return result, err
}
