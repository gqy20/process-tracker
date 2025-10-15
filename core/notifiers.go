package core

// Notifier is the interface for all notification methods
type Notifier interface {
	Send(title, content string) error
}

// NotifierConfig represents configuration for a notifier
type NotifierConfig struct {
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config,omitempty"`
}

// NotifierFactory creates notifiers based on configuration
func NewNotifier(notifierType string, config map[string]interface{}) (Notifier, error) {
	switch notifierType {
	case "webhook":
		return NewWebhookNotifier(config), nil
	case "dingtalk":
		return NewDingTalkNotifier(config), nil
	case "wechat":
		return NewWeChatNotifier(config), nil
	case "feishu":
		return NewFeishuNotifier(config), nil
	default:
		return nil, nil // Silently ignore unknown types
	}
}
