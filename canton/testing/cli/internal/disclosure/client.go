package disclosure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client is the CLI-side client for a disclosure service (Service, above). Used by
// cmd/ntt-playground/remote.go's prepareRemoteSeam when --disclosure-service-url is set (the
// plan's §5.3), and directly by anything that only needs the generic /v1/disclosures layer.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client // nil → http.DefaultClient
	Logf       func(string, ...any)
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

// Disclosures calls GET /v1/disclosures?template=... once per template, returning the served
// contracts and the offset they were valid at. A non-2xx response (notably 403, when a
// requested template is not in the service's allow-list) is returned as an error carrying the
// status and body verbatim, so a caller can surface the offending template name.
func (c *Client) Disclosures(ctx context.Context, templates []string) ([]Contract, int64, error) {
	u, err := url.Parse(c.BaseURL + "/v1/disclosures")
	if err != nil {
		return nil, 0, fmt.Errorf("disclosure: client: parse base URL: %w", err)
	}
	q := u.Query()
	for _, t := range templates {
		q.Add("template", t)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("disclosure: client: build disclosures request: %w", err)
	}
	c.logf("disclosure: client: GET %s", u.String())

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("disclosure: client: disclosures request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("disclosure: client: read disclosures response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("disclosure: client: disclosures: HTTP %d: %s", resp.StatusCode, body)
	}

	var out disclosuresResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, 0, fmt.Errorf("disclosure: client: parse disclosures response: %w\nraw: %s", err, body)
	}
	return out.Contracts, out.ActiveAtOffset, nil
}

// Seam calls POST /v1/seam/{name}, marshaling input as the request body, and returns the
// script's RemoteSeam output verbatim as raw JSON -- the CLI never parses it, only relays it
// into the next `dpm script` call's input (matching how transferOutInput.Remote is already
// handled today). A non-2xx response is returned as an error carrying the status and body.
func (c *Client) Seam(ctx context.Context, name string, input any) (json.RawMessage, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("disclosure: client: marshal seam %s input: %w", name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/seam/"+name, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("disclosure: client: build seam %s request: %w", name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.logf("disclosure: client: POST /v1/seam/%s", name)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("disclosure: client: seam %s request: %w", name, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("disclosure: client: read seam %s response: %w", name, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("disclosure: client: seam %s: HTTP %d: %s", name, resp.StatusCode, respBody)
	}
	return json.RawMessage(respBody), nil
}
