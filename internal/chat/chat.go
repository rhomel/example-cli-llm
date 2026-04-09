package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	HTTPClient *http.Client
}

type Request struct {
	BaseURL      string
	APIKey       string
	Model        string
	SystemPrompt string
	UserPrompt   string
	N            int
}

func NewClient() Client {
	return Client{
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c Client) Complete(ctx context.Context, req Request) ([]string, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	payload := map[string]interface{}{
		"model": req.Model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
	}
	if req.N > 1 {
		payload["n"] = req.N
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(req.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error map[string]interface{} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if message, ok := apiErr.Error["message"].(string); ok && message != "" {
			return nil, fmt.Errorf("chat completions request failed: %s", message)
		}
		return nil, fmt.Errorf("chat completions request failed: status %s", resp.Status)
	}

	var payloadResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payloadResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]string, 0, len(payloadResp.Choices))
	for _, choice := range payloadResp.Choices {
		content := strings.TrimSpace(choice.Message.Content)
		if content == "" {
			continue
		}
		results = append(results, content)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("chat completions response contained no choices")
	}
	return results, nil
}
