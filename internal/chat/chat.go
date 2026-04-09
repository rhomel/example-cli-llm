package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	Temperature  float64
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
	model := strings.TrimSpace(req.Model)
	if model == "" {
		discovered, err := c.discoverModel(ctx, req.BaseURL, req.APIKey)
		if err != nil {
			return nil, err
		}
		model = discovered
	}
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
	}
	if req.N > 1 {
		payload["n"] = req.N
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(req.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if strings.TrimSpace(req.APIKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
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
		message, _ := apiErr.Error["message"].(string)
		if message == "" {
			return nil, fmt.Errorf("chat completions request failed: status %s", resp.Status)
		}
		return nil, fmt.Errorf("chat completions request failed: %s", message)
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

func (c Client) discoverModel(ctx context.Context, baseURL, apiKey string) (string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return "", fmt.Errorf("create model discovery request: %w", err)
	}
	if strings.TrimSpace(apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("discover model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		trimmed := strings.TrimSpace(string(body))
		if trimmed == "" {
			return "", fmt.Errorf("discover model: status %s", resp.Status)
		}
		return "", fmt.Errorf("discover model: status %s: %s", resp.Status, trimmed)
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode model discovery response: %w", err)
	}
	for _, model := range payload.Data {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		return id, nil
	}
	return "", fmt.Errorf("discover model: no models returned")
}
