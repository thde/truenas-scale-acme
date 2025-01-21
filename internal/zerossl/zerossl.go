package zerossl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/caddyserver/certmagic"
	"github.com/caddyserver/zerossl"
	"github.com/mholt/acmez/v3/acme"
)

// EABCredentials generates ZeroSSL EAB credentials for the primary contact email
// on the issuer. It should only be used if the CA endpoint is ZeroSSL. An email address is required.
// https://github.com/caddyserver/caddy/blob/master/modules/caddytls/acmeissuer.go#L269
func EABCredentials(ctx context.Context, email string, acct acme.Account) (*acme.EAB, acme.Account, error) {
	if strings.TrimSpace(email) == "" {
		return nil, acme.Account{}, fmt.Errorf("your email address is required to use ZeroSSL's ACME endpoint")
	}

	if len(acct.Contact) == 0 {
		// we borrow the email from config or the default email, so ensure it's saved with the account
		acct.Contact = []string{"mailto:" + email}
	}

	endpoint := zerossl.BaseURL + "/acme/eab-credentials-email"
	form := url.Values{"email": []string{email}}
	body := strings.NewReader(form.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, acct, fmt.Errorf("forming request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", certmagic.UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, acct, fmt.Errorf("performing EAB credentials request: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
		Error   struct {
			Code int    `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
		EABKID     string `json:"eab_kid"`
		EABHMACKey string `json:"eab_hmac_key"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, acct, fmt.Errorf("decoding API response: %v", err)
	}
	if result.Error.Code != 0 {
		// do this check first because ZeroSSL's API returns 200 on errors
		return nil, acct, fmt.Errorf("failed getting EAB credentials: HTTP %d: %s (code %d)",
			resp.StatusCode, result.Error.Type, result.Error.Code)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, acct, fmt.Errorf("failed getting EAB credentials: HTTP %d", resp.StatusCode)
	}

	return &acme.EAB{
		KeyID:  result.EABKID,
		MACKey: result.EABHMACKey,
	}, acct, nil
}
