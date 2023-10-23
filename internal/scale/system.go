package scale

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type SystemGeneral struct {
	ID                   int         `json:"id,omitempty"`
	Language             string      `json:"language,omitempty"`
	KDBMap               string      `json:"kbdmap,omitempty"`
	Birthday             Time        `json:"birthday,omitempty"`
	Timezone             string      `json:"timezone,omitempty"`
	Wizardshown          bool        `json:"wizardshown,omitempty"`
	CrashReporting       bool        `json:"crash_reporting,omitempty"`
	UsageCollection      bool        `json:"usage_collection,omitempty"`
	DSAuth               bool        `json:"ds_auth,omitempty"`
	UIAddress            []string    `json:"ui_address,omitempty"`
	UIV6Address          []string    `json:"ui_v6address,omitempty"`
	UIAllowlist          []any       `json:"ui_allowlist,omitempty"`
	UIPort               int         `json:"ui_port,omitempty"`
	UIHttpsport          int         `json:"ui_httpsport,omitempty"`
	UIHttpsredirect      bool        `json:"ui_httpsredirect,omitempty"`
	UIHttpsprotocols     []string    `json:"ui_httpsprotocols,omitempty"`
	UIXFrameOptions      string      `json:"ui_x_frame_options,omitempty"`
	UIConsolemsg         bool        `json:"ui_consolemsg,omitempty"`
	UICertificate        Certificate `json:"ui_certificate,omitempty"`
	CrashReportingIsSet  bool        `json:"crash_reporting_is_set,omitempty"`
	UsageCollectionIsSet bool        `json:"usage_collection_is_set,omitempty"`
}

// SystemGeneral returns general system config
func (c *Client) SystemGeneral(ctx context.Context) (*SystemGeneral, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "system/general", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("system general error: %w", err)
	}

	general := SystemGeneral{}
	_, err = c.doJSON(req, &general)
	return &general, err
}

type SystemGeneralUpdateParams struct {
	UICertificate   int `json:"ui_certificate,omitempty"`
	UIRestartDelay  int `json:"ui_restart_delay,omitempty"`
	RollbackTimeout int `json:"rollback_timeout,omitempty"`
}

// SystemGeneralUpdate updates general system config
func (c *Client) SystemGeneralUpdate(ctx context.Context, params SystemGeneralUpdateParams) (error) {
	if params.UIRestartDelay == 0 {
		params.UIRestartDelay = 2
	}
	if params.RollbackTimeout == 0 {
		params.RollbackTimeout = 30
	}

	req, err := c.newRequest(ctx, http.MethodPut, "system/general", nil, params)
	if err != nil {
		return fmt.Errorf("system general error: %w", err)
	}

	_, err = c.do(req)
	if err != nil {
		return err
	}

	time.Sleep(time.Duration(params.UIRestartDelay + 1) * time.Second) // wait for restart delay

	timeout := time.After(time.Duration(params.RollbackTimeout) * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return errors.New("timed out")
		case <-ticker.C:
			err := c.SystemGeneralCheckIn(ctx)
			if err != nil {
				continue
			}

			return nil
		}
	}
}

func (c *Client) SystemGeneralCheckIn(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "system/general/checkin", nil, nil)
	if err != nil {
		return fmt.Errorf("system general error: %w", err)
	}

	_, err = c.do(req)

	return err
}
