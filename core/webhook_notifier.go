package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// WebhookNotifier sends notifications via generic HTTP webhook
type WebhookNotifier struct {
	URL     string
	Method  string
	Headers map[string]string
	Timeout time.Duration
}

// NewWebhookNotifier creates a new webhook notifier
func NewWebhookNotifier(config map[string]interface{}) *WebhookNotifier {
	wn := &WebhookNotifier{
		Method:  "POST",
		Timeout: 10 * time.Second,
		Headers: make(map[string]string),
	}

	if url, ok := config["url"].(string); ok {
		wn.URL = url
	}
	
	// Support environment variable for URL
	if wn.URL == "" || wn.URL == "${WEBHOOK_URL}" {
		if envURL := os.Getenv("WEBHOOK_URL"); envURL != "" {
			wn.URL = envURL
		}
	}
	
	if method, ok := config["method"].(string); ok {
		wn.Method = method
	}
	if headers, ok := config["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if str, ok := v.(string); ok {
				wn.Headers[k] = str
			}
		}
	}

	// Set default Content-Type if not specified
	if _, ok := wn.Headers["Content-Type"]; !ok {
		wn.Headers["Content-Type"] = "application/json"
	}

	return wn
}

// Send sends a notification via webhook
func (wn *WebhookNotifier) Send(title, content string) error {
	if wn.URL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	// Prepare payload
	payload := map[string]interface{}{
		"title":     title,
		"content":   content,
		"timestamp": time.Now().Unix(),
		"source":    "process-tracker",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	req, err := http.NewRequest(wn.Method, wn.URL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range wn.Headers {
		req.Header.Set(k, v)
	}

	// Send request
	client := &http.Client{Timeout: wn.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
