package core

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// DingTalkNotifier sends notifications to DingTalk robot
type DingTalkNotifier struct {
	WebhookURL string
	Secret     string
	Timeout    time.Duration
}

// NewDingTalkNotifier creates a new DingTalk notifier
func NewDingTalkNotifier(config map[string]interface{}) *DingTalkNotifier {
	dn := &DingTalkNotifier{
		Timeout: 10 * time.Second,
	}

	if webhookURL, ok := config["webhook_url"].(string); ok {
		dn.WebhookURL = webhookURL
	}
	if secret, ok := config["secret"].(string); ok {
		dn.Secret = secret
	}

	return dn
}

// Send sends a notification to DingTalk
func (dn *DingTalkNotifier) Send(title, content string) error {
	if dn.WebhookURL == "" {
		return fmt.Errorf("DingTalk webhook URL is empty")
	}

	// Generate signature if secret is provided
	requestURL := dn.WebhookURL
	if dn.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := dn.generateSign(timestamp)
		requestURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", dn.WebhookURL, timestamp, url.QueryEscape(sign))
	}

	// Prepare markdown message
	message := fmt.Sprintf("### %s\n\n%s\n\n---\n\n时间: %s",
		title,
		content,
		time.Now().Format("2006-01-02 15:04:05"))

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  message,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Send request
	req, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: dn.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send DingTalk notification: %w", err)
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
		return fmt.Errorf("DingTalk error: %s (code: %d)", result.ErrMsg, result.ErrCode)
	}

	return nil
}

// generateSign generates signature for DingTalk webhook
func (dn *DingTalkNotifier) generateSign(timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, dn.Secret)
	h := hmac.New(sha256.New, []byte(dn.Secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
