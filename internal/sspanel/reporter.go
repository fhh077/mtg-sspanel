package sspanel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

type TrafficReport struct {
	UserID   string `json:"user_id"`
	Upload   int64  `json:"u"`
	Download int64  `json:"d"`
}

func (c *Client) ReportTraffic(ctx context.Context, records []TrafficReport) error {
	u, _ := url.Parse(c.baseURL + "/mod_mu/users/traffic")
	q := u.Query()
	q.Set("node_id", c.nodeID)
	u.RawQuery = q.Encode()

	body, err := json.Marshal(struct {
		Data []TrafficReport `json:"data"`
	}{Data: records})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}