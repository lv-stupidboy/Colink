package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AuthClient 认证客户端
type AuthClient struct {
	apiURL        string
	invocationID  string
	callbackToken string
	httpClient    *http.Client
}

// NewAuthClient 创建认证客户端
func NewAuthClient(apiURL, invocationID, callbackToken string) *AuthClient {
	return &AuthClient{
		apiURL:        apiURL,
		invocationID:  invocationID,
		callbackToken: callbackToken,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

// CallAPI 调用 ISDP API（带认证）
func (c *AuthClient) CallAPI(method, path string, body interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s/api/callbacks%s", c.apiURL, path)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置认证头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Invocation-ID", c.invocationID)
	req.Header.Set("X-Callback-Token", c.callbackToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	return respBody, nil
}

// ValidateToken 验证 Token 是否有效
func (c *AuthClient) ValidateToken() error {
	_, err := c.CallAPI("GET", "/validate", nil)
	return err
}