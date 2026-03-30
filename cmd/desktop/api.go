package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

func apiGet(url, accountID string) (map[string]any, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if accountID != "" {
		req.Header.Set("Authorization", "Account "+accountID)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	json.Unmarshal(body, &result)

	if resp.StatusCode >= 400 {
		errMsg := "unknown error"
		if e, ok := result["error"].(string); ok {
			errMsg = e
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
	}

	return result, nil
}

func apiPost(url string, body any, accountID string) (map[string]any, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if accountID != "" {
		req.Header.Set("Authorization", "Account "+accountID)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]any
	json.Unmarshal(respBody, &result)

	if resp.StatusCode >= 400 {
		errMsg := "unknown error"
		if e, ok := result["error"].(string); ok {
			errMsg = e
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
	}

	return result, nil
}
