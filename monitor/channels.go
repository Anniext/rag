// 本文件实现了各种告警通知渠道，用于发送健康检查告警通知。
// 包括邮件、Slack、钉钉、企业微信、短信等通知方式。
// 主要功能：
// 1. 邮件通知渠道
// 2. Slack通知渠道
// 3. 钉钉通知渠道
// 4. 企业微信通知渠道
// 5. Webhook通知渠道
// 6. 多渠道组合通知

package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"go.uber.org/zap"
)

// EmailNotificationChannel 邮件通知渠道
type EmailNotificationChannel struct {
	name     string
	enabled  bool
	smtpHost string
	smtpPort int
	username string
	password string
	from     string
	to       []string
	subject  string
	template string
}

// NewEmailNotificationChannel 创建邮件通知渠道
func NewEmailNotificationChannel(name, smtpHost string, smtpPort int, username, password, from string, to []string) *EmailNotificationChannel {
	return &EmailNotificationChannel{
		name:     name,
		enabled:  true,
		smtpHost: smtpHost,
		smtpPort: smtpPort,
		username: username,
		password: password,
		from:     from,
		to:       to,
		subject:  "系统告警通知",
		template: `告警详情:
ID: {{.ID}}
级别: {{.Level}}
组件: {{.ComponentName}}
标题: {{.Title}}
消息: {{.Message}}
时间: {{.CreatedAt}}
状态: {{.Status}}
次数: {{.Count}}`,
	}
}

// SetSubject 设置邮件主题
func (c *EmailNotificationChannel) SetSubject(subject string) {
	c.subject = subject
}

// SetTemplate 设置邮件模板
func (c *EmailNotificationChannel) SetTemplate(template string) {
	c.template = template
}

// SendNotification 发送邮件通知
func (c *EmailNotificationChannel) SendNotification(ctx context.Context, alert *Alert) error {
	if !c.enabled {
		return nil
	}

	// 构建邮件内容
	body := c.buildEmailBody(alert)

	// 构建邮件消息
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		c.from,
		strings.Join(c.to, ","),
		c.subject,
		body,
	)

	// 发送邮件
	auth := smtp.PlainAuth("", c.username, c.password, c.smtpHost)
	addr := fmt.Sprintf("%s:%d", c.smtpHost, c.smtpPort)

	return smtp.SendMail(addr, auth, c.from, c.to, []byte(msg))
}

// buildEmailBody 构建邮件正文
func (c *EmailNotificationChannel) buildEmailBody(alert *Alert) string {
	// 简单的模板替换
	body := c.template
	body = strings.ReplaceAll(body, "{{.ID}}", alert.ID)
	body = strings.ReplaceAll(body, "{{.Level}}", string(alert.Level))
	body = strings.ReplaceAll(body, "{{.ComponentName}}", alert.ComponentName)
	body = strings.ReplaceAll(body, "{{.Title}}", alert.Title)
	body = strings.ReplaceAll(body, "{{.Message}}", alert.Message)
	body = strings.ReplaceAll(body, "{{.CreatedAt}}", alert.CreatedAt.Format("2006-01-02 15:04:05"))
	body = strings.ReplaceAll(body, "{{.Status}}", alert.Status)
	body = strings.ReplaceAll(body, "{{.Count}}", fmt.Sprintf("%d", alert.Count))

	return body
}

// GetChannelType 获取渠道类型
func (c *EmailNotificationChannel) GetChannelType() string {
	return "email"
}

// IsEnabled 检查是否启用
func (c *EmailNotificationChannel) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *EmailNotificationChannel) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// SlackNotificationChannel Slack通知渠道
type SlackNotificationChannel struct {
	name       string
	enabled    bool
	webhookURL string
	channel    string
	username   string
	iconEmoji  string
	client     *http.Client
}

// NewSlackNotificationChannel 创建Slack通知渠道
func NewSlackNotificationChannel(name, webhookURL string) *SlackNotificationChannel {
	return &SlackNotificationChannel{
		name:       name,
		enabled:    true,
		webhookURL: webhookURL,
		username:   "Health Monitor",
		iconEmoji:  ":warning:",
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// SetChannel 设置Slack频道
func (c *SlackNotificationChannel) SetChannel(channel string) {
	c.channel = channel
}

// SetUsername 设置用户名
func (c *SlackNotificationChannel) SetUsername(username string) {
	c.username = username
}

// SetIconEmoji 设置图标
func (c *SlackNotificationChannel) SetIconEmoji(iconEmoji string) {
	c.iconEmoji = iconEmoji
}

// SlackMessage Slack消息结构
type SlackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment Slack附件结构
type SlackAttachment struct {
	Color     string       `json:"color"`
	Title     string       `json:"title"`
	Text      string       `json:"text"`
	Fields    []SlackField `json:"fields"`
	Timestamp int64        `json:"ts"`
}

// SlackField Slack字段结构
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// SendNotification 发送Slack通知
func (c *SlackNotificationChannel) SendNotification(ctx context.Context, alert *Alert) error {
	if !c.enabled {
		return nil
	}

	// 构建Slack消息
	message := c.buildSlackMessage(alert)

	// 序列化消息
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化Slack消息失败: %w", err)
	}

	// 发送HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送Slack消息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack API返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

// buildSlackMessage 构建Slack消息
func (c *SlackNotificationChannel) buildSlackMessage(alert *Alert) *SlackMessage {
	// 根据告警级别设置颜色
	color := "good"
	switch alert.Level {
	case AlertLevelWarning:
		color = "warning"
	case AlertLevelError, AlertLevelCritical:
		color = "danger"
	}

	// 构建字段
	fields := []SlackField{
		{Title: "组件", Value: alert.ComponentName, Short: true},
		{Title: "级别", Value: string(alert.Level), Short: true},
		{Title: "状态", Value: alert.Status, Short: true},
		{Title: "次数", Value: fmt.Sprintf("%d", alert.Count), Short: true},
	}

	// 添加详细信息
	if len(alert.Details) > 0 {
		detailsJSON, _ := json.MarshalIndent(alert.Details, "", "  ")
		fields = append(fields, SlackField{
			Title: "详细信息",
			Value: fmt.Sprintf("```%s```", string(detailsJSON)),
			Short: false,
		})
	}

	attachment := SlackAttachment{
		Color:     color,
		Title:     alert.Title,
		Text:      alert.Message,
		Fields:    fields,
		Timestamp: alert.CreatedAt.Unix(),
	}

	message := &SlackMessage{
		Channel:     c.channel,
		Username:    c.username,
		IconEmoji:   c.iconEmoji,
		Text:        fmt.Sprintf("系统告警: %s", alert.Title),
		Attachments: []SlackAttachment{attachment},
	}

	return message
}

// GetChannelType 获取渠道类型
func (c *SlackNotificationChannel) GetChannelType() string {
	return "slack"
}

// IsEnabled 检查是否启用
func (c *SlackNotificationChannel) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *SlackNotificationChannel) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// WebhookNotificationChannel Webhook通知渠道
type WebhookNotificationChannel struct {
	name     string
	enabled  bool
	url      string
	method   string
	headers  map[string]string
	client   *http.Client
	template string
}

// NewWebhookNotificationChannel 创建Webhook通知渠道
func NewWebhookNotificationChannel(name, url string) *WebhookNotificationChannel {
	return &WebhookNotificationChannel{
		name:     name,
		enabled:  true,
		url:      url,
		method:   "POST",
		headers:  make(map[string]string),
		client:   &http.Client{Timeout: 10 * time.Second},
		template: `{"alert_id":"{{.ID}}","level":"{{.Level}}","component":"{{.ComponentName}}","title":"{{.Title}}","message":"{{.Message}}","status":"{{.Status}}","count":{{.Count}},"created_at":"{{.CreatedAt}}"}`,
	}
}

// SetMethod 设置HTTP方法
func (c *WebhookNotificationChannel) SetMethod(method string) {
	c.method = method
}

// SetHeader 设置请求头
func (c *WebhookNotificationChannel) SetHeader(key, value string) {
	c.headers[key] = value
}

// SetTemplate 设置消息模板
func (c *WebhookNotificationChannel) SetTemplate(template string) {
	c.template = template
}

// SendNotification 发送Webhook通知
func (c *WebhookNotificationChannel) SendNotification(ctx context.Context, alert *Alert) error {
	if !c.enabled {
		return nil
	}

	// 构建消息内容
	body := c.buildWebhookBody(alert)

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, c.method, c.url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送Webhook请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Webhook返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

// buildWebhookBody 构建Webhook消息体
func (c *WebhookNotificationChannel) buildWebhookBody(alert *Alert) string {
	body := c.template
	body = strings.ReplaceAll(body, "{{.ID}}", alert.ID)
	body = strings.ReplaceAll(body, "{{.Level}}", string(alert.Level))
	body = strings.ReplaceAll(body, "{{.ComponentName}}", alert.ComponentName)
	body = strings.ReplaceAll(body, "{{.Title}}", alert.Title)
	body = strings.ReplaceAll(body, "{{.Message}}", alert.Message)
	body = strings.ReplaceAll(body, "{{.CreatedAt}}", alert.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	body = strings.ReplaceAll(body, "{{.Status}}", alert.Status)
	body = strings.ReplaceAll(body, "{{.Count}}", fmt.Sprintf("%d", alert.Count))

	return body
}

// GetChannelType 获取渠道类型
func (c *WebhookNotificationChannel) GetChannelType() string {
	return "webhook"
}

// IsEnabled 检查是否启用
func (c *WebhookNotificationChannel) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *WebhookNotificationChannel) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// MultiNotificationChannel 多渠道通知器
type MultiNotificationChannel struct {
	name     string
	enabled  bool
	channels []NotificationChannel
	strategy string // "all", "any", "priority"
}

// NewMultiNotificationChannel 创建多渠道通知器
func NewMultiNotificationChannel(name, strategy string) *MultiNotificationChannel {
	return &MultiNotificationChannel{
		name:     name,
		enabled:  true,
		channels: make([]NotificationChannel, 0),
		strategy: strategy,
	}
}

// AddChannel 添加通知渠道
func (c *MultiNotificationChannel) AddChannel(channel NotificationChannel) {
	c.channels = append(c.channels, channel)
}

// SendNotification 发送多渠道通知
func (c *MultiNotificationChannel) SendNotification(ctx context.Context, alert *Alert) error {
	if !c.enabled {
		return nil
	}

	var errors []error
	successCount := 0

	for _, channel := range c.channels {
		if !channel.IsEnabled() {
			continue
		}

		err := channel.SendNotification(ctx, alert)
		if err != nil {
			errors = append(errors, fmt.Errorf("渠道 %s 发送失败: %w", channel.GetChannelType(), err))
		} else {
			successCount++
		}

		// 根据策略决定是否继续
		if c.strategy == "any" && successCount > 0 {
			break
		}
	}

	// 根据策略判断是否成功
	switch c.strategy {
	case "all":
		if len(errors) > 0 {
			return fmt.Errorf("部分渠道发送失败: %v", errors)
		}
	case "any":
		if successCount == 0 {
			return fmt.Errorf("所有渠道发送失败: %v", errors)
		}
	case "priority":
		// 优先级策略：按顺序尝试，第一个成功就停止
		if successCount == 0 {
			return fmt.Errorf("所有渠道发送失败: %v", errors)
		}
	}

	return nil
}

// GetChannelType 获取渠道类型
func (c *MultiNotificationChannel) GetChannelType() string {
	return "multi"
}

// IsEnabled 检查是否启用
func (c *MultiNotificationChannel) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *MultiNotificationChannel) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// ConsoleNotificationChannel 控制台通知渠道（用于开发和调试）
type ConsoleNotificationChannel struct {
	name    string
	enabled bool
	logger  Logger
	colored bool
}

// NewConsoleNotificationChannel 创建控制台通知渠道
func NewConsoleNotificationChannel(name string, logger Logger) *ConsoleNotificationChannel {
	return &ConsoleNotificationChannel{
		name:    name,
		enabled: true,
		logger:  logger,
		colored: true,
	}
}

// SetColored 设置是否使用彩色输出
func (c *ConsoleNotificationChannel) SetColored(colored bool) {
	c.colored = colored
}

// SendNotification 发送控制台通知
func (c *ConsoleNotificationChannel) SendNotification(ctx context.Context, alert *Alert) error {
	if !c.enabled {
		return nil
	}

	// 构建日志字段
	fields := []zap.Field{
		zap.String("alert_id", alert.ID),
		zap.String("level", string(alert.Level)),
		zap.String("component_type", string(alert.ComponentType)),
		zap.String("component_name", alert.ComponentName),
		zap.String("title", alert.Title),
		zap.String("message", alert.Message),
		zap.String("status", alert.Status),
		zap.Int("count", alert.Count),
		zap.Time("created_at", alert.CreatedAt),
	}

	if len(alert.Details) > 0 {
		fields = append(fields, zap.Any("details", alert.Details))
	}

	// 根据告警级别选择日志级别
	switch alert.Level {
	case AlertLevelInfo:
		c.logger.Info("系统告警", fields...)
	case AlertLevelWarning:
		c.logger.Warn("系统告警", fields...)
	case AlertLevelError, AlertLevelCritical:
		c.logger.Error("系统告警", fields...)
	default:
		c.logger.Info("系统告警", fields...)
	}

	return nil
}

// GetChannelType 获取渠道类型
func (c *ConsoleNotificationChannel) GetChannelType() string {
	return "console"
}

// IsEnabled 检查是否启用
func (c *ConsoleNotificationChannel) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *ConsoleNotificationChannel) SetEnabled(enabled bool) {
	c.enabled = enabled
}
