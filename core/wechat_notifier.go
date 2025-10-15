package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WeChatNotifier sends notifications to WeChat Work robot
type WeChatNotifier struct {
	WebhookURL string
	Timeout    time.Duration
}

// NewWeChatNotifier creates a new WeChat Work notifier
func NewWeChatNotifier(config map[string]interface{}) *WeChatNotifier {
	wn := &WeChatNotifier{
		Timeout: 10 * time.Second,
	}

	if webhookURL, ok := config["webhook_url"].(string); ok {
		wn.WebhookURL = webhookURL
	}

	return wn
}

// Send sends a notification to WeChat Work
func (wn *WeChatNotifier) Send(title, content string) error {
	if wn.WebhookURL == "" {
		return fmt.Errorf("WeChat webhook URL is empty")
	}

	// Prepare markdown message
	message := fmt.Sprintf("**%s**\n\n%s\n\n<font color=\"info\">%s</font>",
		title,
		content,
		time.Now().Format("2006-01-02 15:04:05"))

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": message,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Send request
	req, err := http.NewRequest("POST", wn.WebhookURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: wn.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send WeChat notification: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("WeChat error: %s (code: %d)", result.ErrMsg, result.ErrCode)
	}

	return nil
}
