package core

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// AlertRule defines an alert rule
type AlertRule struct {
	Name      string   `yaml:"name"`
	Metric    string   `yaml:"metric"`     // cpu_percent, memory_mb
	Threshold float64  `yaml:"threshold"`  // Threshold value
	Duration  int      `yaml:"duration"`   // Duration in seconds before alerting
	Channels  []string `yaml:"channels"`   // List of notifier channels
	Process   string   `yaml:"process"`    // Optional: specific process name
	Enabled   bool     `yaml:"enabled"`    // Whether the rule is enabled
}

// AlertState tracks the state of an alert
type AlertState struct {
	Rule        *AlertRule
	StartTime   time.Time
	Count       int       // Number of times threshold exceeded
	LastNotify  time.Time // Last notification time
	Suppressed  bool      // Whether alert is suppressed
	CurrentValue float64  // Current metric value
}

// AlertManager manages alert rules and notifications
type AlertManager struct {
	rules     []AlertRule
	notifiers map[string]Notifier
	states    map[string]*AlertState
	mu        sync.RWMutex
	
	// Configuration
	suppressDuration time.Duration // Suppress repeat notifications
}

// AlertConfig represents alert configuration
type AlertConfig struct {
	Enabled          bool              `yaml:"enabled"`
	Rules            []AlertRule       `yaml:"rules"`
	SuppressDuration int               `yaml:"suppress_duration"` // In minutes
}

// NotifiersConfig represents notifiers configuration
type NotifiersConfig map[string]map[string]interface{}

// NewAlertManager creates a new alert manager
func NewAlertManager(config AlertConfig, notifiersConfig NotifiersConfig) *AlertManager {
	am := &AlertManager{
		rules:            config.Rules,
		notifiers:        make(map[string]Notifier),
		states:           make(map[string]*AlertState),
		suppressDuration: time.Duration(config.SuppressDuration) * time.Minute,
	}

	// Default suppress duration
	if am.suppressDuration == 0 {
		am.suppressDuration = 30 * time.Minute
	}

	// Create notifiers
	for name, cfg := range notifiersConfig {
		notifier, err := NewNotifier(name, cfg)
		if err != nil {
			log.Printf("警告: 创建通知器 %s 失败: %v", name, err)
			continue
		}
		if notifier != nil {
			am.notifiers[name] = notifier
		}
	}

	log.Printf("告警管理器初始化: %d个规则, %d个通知器", len(am.rules), len(am.notifiers))
	return am
}

// Evaluate evaluates alert rules against current metrics
func (am *AlertManager) Evaluate(records []ResourceRecord) {
	if len(records) == 0 {
		return
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	// Evaluate each rule
	for _, rule := range am.rules {
		if !rule.Enabled {
			continue
		}

		// Get metric value
		value := am.getMetricValue(records, rule.Metric, rule.Process)

		// Check threshold
		if value > rule.Threshold {
			am.handleAlert(rule, value)
		} else {
			am.clearAlert(rule.Name)
		}
	}
}

// getMetricValue calculates metric value from records
func (am *AlertManager) getMetricValue(records []ResourceRecord, metric, processName string) float64 {
	var total float64
	var count int

	for _, r := range records {
		// Filter by process name if specified
		if processName != "" && r.Name != processName {
			continue
		}

		switch metric {
		case "cpu_percent":
			total += r.CPUPercent
			count++
		case "memory_mb":
			total += r.MemoryMB
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return total / float64(count)
}

// handleAlert handles an alert condition
func (am *AlertManager) handleAlert(rule AlertRule, value float64) {
	state, exists := am.states[rule.Name]
	if !exists {
		state = &AlertState{
			Rule:      &rule,
			StartTime: time.Now(),
		}
		am.states[rule.Name] = state
	}

	state.Count++
	state.CurrentValue = value

	// Check if duration threshold is met
	duration := time.Since(state.StartTime).Seconds()
	if duration < float64(rule.Duration) {
		return // Not yet time to alert
	}

	// Check if suppression period has passed
	if state.Suppressed && time.Since(state.LastNotify) < am.suppressDuration {
		return // Still in suppression period
	}

	// Send alert
	am.sendAlert(rule, value, duration)
	
	state.LastNotify = time.Now()
	state.Suppressed = true
}

// clearAlert clears an alert state
func (am *AlertManager) clearAlert(ruleName string) {
	if state, exists := am.states[ruleName]; exists {
		// If was suppressed, send recovery notification
		if state.Suppressed {
			am.sendRecovery(state.Rule, state.CurrentValue)
		}
		delete(am.states, ruleName)
	}
}

// sendAlert sends alert notification
func (am *AlertManager) sendAlert(rule AlertRule, value, duration float64) {
	title := fmt.Sprintf("🚨 告警: %s", rule.Name)
	content := fmt.Sprintf(
		"**指标**: %s\n**当前值**: %.2f\n**阈值**: %.2f\n**持续时长**: %.0f秒",
		rule.Metric,
		value,
		rule.Threshold,
		duration,
	)

	if rule.Process != "" {
		content += fmt.Sprintf("\n**进程**: %s", rule.Process)
	}

	// Send to all configured channels
	for _, channel := range rule.Channels {
		notifier, ok := am.notifiers[channel]
		if !ok {
			log.Printf("警告: 未找到通知器 %s", channel)
			continue
		}

		if err := notifier.Send(title, content); err != nil {
			log.Printf("发送告警失败 (通知器: %s): %v", channel, err)
		} else {
			log.Printf("告警已发送: %s -> %s", rule.Name, channel)
		}
	}
}

// sendRecovery sends recovery notification
func (am *AlertManager) sendRecovery(rule *AlertRule, lastValue float64) {
	title := fmt.Sprintf("✅ 恢复: %s", rule.Name)
	content := fmt.Sprintf(
		"**指标**: %s\n**上次值**: %.2f\n**阈值**: %.2f\n**状态**: 已恢复正常",
		rule.Metric,
		lastValue,
		rule.Threshold,
	)

	if rule.Process != "" {
		content += fmt.Sprintf("\n**进程**: %s", rule.Process)
	}

	// Send to all configured channels
	for _, channel := range rule.Channels {
		notifier, ok := am.notifiers[channel]
		if !ok {
			continue
		}

		if err := notifier.Send(title, content); err != nil {
			log.Printf("发送恢复通知失败 (通知器: %s): %v", channel, err)
		}
	}
}

// GetActiveAlerts returns currently active alerts
func (am *AlertManager) GetActiveAlerts() []AlertState {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var alerts []AlertState
	for _, state := range am.states {
		alerts = append(alerts, *state)
	}
	return alerts
}

// TestNotifier tests a notifier by sending a test message
func (am *AlertManager) TestNotifier(channel string) error {
	am.mu.RLock()
	notifier, ok := am.notifiers[channel]
	am.mu.RUnlock()

	if !ok {
		return fmt.Errorf("notifier not found: %s", channel)
	}

	title := "测试通知"
	content := fmt.Sprintf(
		"这是一条来自 Process Tracker 的测试通知\n\n时间: %s",
		time.Now().Format("2006-01-02 15:04:05"),
	)

	return notifier.Send(title, content)
}
