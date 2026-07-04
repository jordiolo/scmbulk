package scm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

const securityRulesPath = "/config/security/v1/security-rules"

type listResponse struct {
	Data  json.RawMessage `json:"data"`
	Total int             `json:"total"`
}

func (c *Client) do(method, reqURL string, body io.Reader) ([]byte, int, error) {
	if err := c.refreshIfNeeded(); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(c.ctx, method, reqURL, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.debug {
		log.Printf("[DEBUG] %s %s", method, reqURL)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, nil
}

// ListSecurityRules returns all rules in the folder for the given position.
func (c *Client) ListSecurityRules(position string) ([]map[string]interface{}, error) {
	var all []map[string]interface{}
	offset := 0
	for {
		reqURL := fmt.Sprintf("%s%s?folder=%s&position=%s&limit=%d&offset=%d",
			BaseURL, securityRulesPath, url.QueryEscape(c.folder),
			url.QueryEscape(position), pageSize, offset)
		body, status, err := c.do(http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("list security rules: HTTP %d: %s", status, string(body))
		}
		var lr listResponse
		if err := json.Unmarshal(body, &lr); err != nil {
			return nil, fmt.Errorf("parsing list response: %w", err)
		}
		var page []map[string]interface{}
		if err := json.Unmarshal(lr.Data, &page); err != nil {
			return nil, fmt.Errorf("parsing rule page: %w", err)
		}
		all = append(all, page...)
		offset += len(page)
		if len(page) == 0 || offset >= lr.Total {
			break
		}
	}
	return all, nil
}

// GetSecurityRule returns the full rule object by id.
func (c *Client) GetSecurityRule(id string) (map[string]interface{}, error) {
	reqURL := fmt.Sprintf("%s%s/%s?folder=%s", BaseURL, securityRulesPath,
		url.PathEscape(id), url.QueryEscape(c.folder))
	body, status, err := c.do(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get security rule %s: HTTP %d: %s", id, status, string(body))
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parsing rule %s: %w", id, err)
	}
	return out, nil
}

// UpdateSecurityRule PUTs the modified payload; id and folder are stripped.
func (c *Client) UpdateSecurityRule(id string, payload map[string]interface{}) error {
	clone := make(map[string]interface{}, len(payload))
	for k, v := range payload {
		clone[k] = v
	}
	delete(clone, "id")
	delete(clone, "folder")

	data, err := json.Marshal(clone)
	if err != nil {
		return fmt.Errorf("serializing rule %s: %w", id, err)
	}
	reqURL := fmt.Sprintf("%s%s/%s?folder=%s", BaseURL, securityRulesPath,
		url.PathEscape(id), url.QueryEscape(c.folder))
	body, status, err := c.do(http.MethodPut, reqURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("update security rule %s: HTTP %d: %s", id, status, string(body))
	}
	return nil
}
