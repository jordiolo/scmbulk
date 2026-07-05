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

// ListRules returns all rules at resourcePath in the folder for the position.
func (c *Client) ListRules(resourcePath, position string) ([]map[string]interface{}, error) {
	var all []map[string]interface{}
	offset := 0
	for {
		reqURL := fmt.Sprintf("%s%s?folder=%s&position=%s&limit=%d&offset=%d",
			BaseURL, resourcePath, url.QueryEscape(c.folder),
			url.QueryEscape(position), pageSize, offset)
		body, status, err := c.do(http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("list rules %s: HTTP %d: %s", resourcePath, status, string(body))
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

// GetRule returns the full rule object by id at resourcePath.
func (c *Client) GetRule(resourcePath, id string) (map[string]interface{}, error) {
	reqURL := fmt.Sprintf("%s%s/%s?folder=%s", BaseURL, resourcePath,
		url.PathEscape(id), url.QueryEscape(c.folder))
	body, status, err := c.do(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get rule %s/%s: HTTP %d: %s", resourcePath, id, status, string(body))
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parsing rule %s: %w", id, err)
	}
	return out, nil
}

// UpdateRule PUTs the modified payload at resourcePath; id and folder stripped.
func (c *Client) UpdateRule(resourcePath, id string, payload map[string]interface{}) error {
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
	reqURL := fmt.Sprintf("%s%s/%s?folder=%s", BaseURL, resourcePath,
		url.PathEscape(id), url.QueryEscape(c.folder))
	body, status, err := c.do(http.MethodPut, reqURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("update rule %s/%s: HTTP %d: %s", resourcePath, id, status, string(body))
	}
	return nil
}
