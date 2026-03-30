package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	accountID  string
	httpClient *http.Client
}

func NewClient(baseURL, accountID string) *Client {
	return &Client{
		baseURL:   baseURL,
		accountID: accountID,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type CreateAccountResponse struct {
	AccountID string    `json:"account_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ServerInfo struct {
	ID        string  `json:"id"`
	Hostname  string  `json:"hostname"`
	PublicIP  string  `json:"public_ip"`
	WGPubKey  string  `json:"wg_pubkey"`
	WGPort    int     `json:"wg_port"`
	Region    Region  `json:"region"`
	Status    string  `json:"status"`
	Load      float64 `json:"load"`
}

type Region struct {
	Code    string `json:"code"`
	City    string `json:"city"`
	Country string `json:"country"`
}

type KeyRegistration struct {
	ID           string `json:"id"`
	ServerID     string `json:"server_id"`
	ServerIP     string `json:"server_ip"`
	ServerPubKey string `json:"server_pubkey"`
	ServerPort   int    `json:"server_port"`
	AllowedIP    string `json:"allowed_ip"`
}

type KeyInfo struct {
	ID        string    `json:"id"`
	PubKey    string    `json:"pubkey"`
	ServerID  string    `json:"server_id"`
	AllowedIP string    `json:"allowed_ip"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *Client) CreateAccount() (*CreateAccountResponse, error) {
	resp, err := c.doRequest("POST", "/v1/accounts", nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result CreateAccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *Client) GetAccount() (*CreateAccountResponse, error) {
	resp, err := c.doRequest("GET", "/v1/accounts/"+c.accountID, nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result CreateAccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *Client) ListServers(region string) ([]ServerInfo, error) {
	path := "/v1/servers"
	if region != "" {
		path += "?region=" + region
	}
	resp, err := c.doRequest("GET", path, nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Servers []ServerInfo `json:"servers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Servers, nil
}

func (c *Client) RegisterKey(pubkey string) (*KeyRegistration, error) {
	body := map[string]string{"pubkey": pubkey}
	resp, err := c.doRequest("POST", "/v1/keys", body, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result KeyRegistration
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *Client) ListKeys() ([]KeyInfo, error) {
	resp, err := c.doRequest("GET", "/v1/keys", nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Keys []KeyInfo `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Keys, nil
}

func (c *Client) RevokeKey(keyID string) error {
	resp, err := c.doRequest("DELETE", "/v1/keys/"+keyID, nil, true)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) doRequest(method, path string, body any, auth bool) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth && c.accountID != "" {
		req.Header.Set("Authorization", "Account "+c.accountID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
	}

	return resp, nil
}
