package sspanel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL string
	nodeID  string
	apiKey  string
	client  *http.Client
}

func NewClient(baseURL, nodeID, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		nodeID:  nodeID,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type User struct {
	ID     string `json:"id"`
	Passwd string `json:"passwd"`
	Port   int    `json:"port"`
	Speed  int    `json:"node_speedlimit"`
}

// 定义心跳响应结构体
type HeartbeatResponse struct {
	Status string `json:"status"`
}

// 定义流量上报结构体
type TrafficReport struct {
	UserID   string `json:"user_id"`
	Upload   int64  `json:"u"`
	Download int64  `json:"d"`
}

type TrafficRequest struct {
	Data []TrafficReport `json:"data"`
}

func (c *Client) FetchUsers(ctx context.Context) ([]User, error) {
	u, _ := url.Parse(c.baseURL + "/mod_mu/users")
	q := u.Query()
	q.Set("node_id", c.nodeID)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}

	return users, nil
}

func (c *Client) SendHeartbeat(ctx context.Context) (string, error) {
	u, _ := url.Parse(c.baseURL + "/check")
	q := u.Query()
	q.Set("id", c.nodeID)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	req.Header.Set("Authorization", "Bearer "+c.apiKey) // 添加认证头

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("heartbeat failed: %d", resp.StatusCode)
	}

	var response HeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to parse heartbeat response: %w", err)
	}

	return response.Status, nil
}

func (c *Client) ReportTraffic(ctx context.Context, reports []TrafficReport) error {
	u, _ := url.Parse(c.baseURL + "/mod_mu/users/traffic")
	
	requestBody := TrafficRequest{Data: reports}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal traffic data: %w", err)
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(jsonData))
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send traffic report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status from traffic report: %d", resp.StatusCode)
	}

	// 检查响应体是否包含成功消息
	var response struct {
		Ret int    `json:"ret"`
		Msg string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to parse traffic report response: %w", err)
	}

	if response.Ret != 1 {
		return fmt.Errorf("traffic report failed: %s", response.Msg)
	}

	return nil
}