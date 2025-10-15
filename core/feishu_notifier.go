package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// FeishuNotifier sends notifications to Feishu (Lark) robot
type FeishuNotifier struct {
	WebhookURL string
	Timeout    time.Duration
}

// NewFeishuNotifier creates a new Feishu notifier
func NewFeishuNotifier(config map[string]interface{}) *FeishuNotifier {
	fn := &FeishuNotifier{
		Timeout: 10 * time.Second,
	}

	if webhookURL, ok := config["webhook_url"].(string); ok {
		fn.WebhookURL = webhookURL
	}

	return fn
}

// Send sends a notification to Feishu
func (fn *FeishuNotifier) Send(title, content string) error {
	if fn.WebhookURL == "" {
		return fmt.Errorf("Feishu webhook URL is empty")
	}

	// Prepare interactive card message
	message := fmt.Sprintf("**%s**\n\n%s\n\n---\n\n🕐 %s",
		title,
		content,
		time.Now().Format("2006-01-02 15:04:05"))

	payload := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": message,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Send request
	req, err := http.NewRequest("POST", fn.WebhookURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: fn.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Feishu notification: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("Feishu error: %s (code: %d)", result.Msg, result.Code)
	}

	return nil
}
