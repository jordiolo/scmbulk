// Package scm is an HTTP client for the Palo Alto Strata Cloud Manager API.
package scm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Overridable endpoints (tests point these at an httptest.Server).
var (
	BaseURL = "https://api.strata.paloaltonetworks.com"
	AuthURL = "https://auth.apps.paloaltonetworks.com/oauth2/access_token"
)

const pageSize = 200

// Client talks to the SCM API for a single tenant/folder.
type Client struct {
	ctx          context.Context
	http         *http.Client
	folder       string
	clientID     string
	clientSecret string
	tsgID        string
	token        string
	tokenExpiry  time.Time
	debug        bool
}

// New authenticates and returns a ready client.
func New(ctx context.Context, clientID, clientSecret, tsgID, folder string, debug bool) (*Client, error) {
	hc := &http.Client{Timeout: 60 * time.Second}
	token, err := fetchToken(hc, clientID, clientSecret, tsgID)
	if err != nil {
		return nil, fmt.Errorf("authenticating against SCM: %w", err)
	}
	if debug {
		log.Println("[DEBUG] authentication OK")
	}
	return &Client{
		ctx:          ctx,
		http:         hc,
		folder:       folder,
		clientID:     clientID,
		clientSecret: clientSecret,
		tsgID:        tsgID,
		token:        token,
		tokenExpiry:  time.Now().Add(900 * time.Second),
		debug:        debug,
	}, nil
}

// Token returns the current bearer token (for tests/debugging).
func (c *Client) Token() string { return c.token }

func fetchToken(hc *http.Client, clientID, clientSecret, tsgID string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", "tsg_id:"+tsgID)

	req, err := http.NewRequest(http.MethodPost, AuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

func (c *Client) refreshIfNeeded() error {
	if time.Until(c.tokenExpiry) > 60*time.Second {
		return nil
	}
	token, err := fetchToken(c.http, c.clientID, c.clientSecret, c.tsgID)
	if err != nil {
		return fmt.Errorf("refreshing token: %w", err)
	}
	c.token = token
	c.tokenExpiry = time.Now().Add(900 * time.Second)
	return nil
}
