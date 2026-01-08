// Package observability provides a Slack callback for alerting on LLM errors and events.
package observability

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// SlackConfig contains configuration for Slack alerting.
type SlackConfig struct {
	WebhookURL       string        // Slack webhook URL
	Channel          string        // Override channel (optional)
	Username         string        // Bot username (default: "LLMux")
	IconEmoji        string        // Bot icon emoji (default: ":robot_face:")
	AlertOnErrors    bool          // Alert on request failures
	AlertOnFallbacks bool          // Alert on fallback events
	AlertOnBudget    bool          // Alert on budget warnings
	MinErrorInterval time.Duration // Minimum interval between error alerts (rate limiting)
	ErrorThreshold   int           // Number of errors before alerting
}

// DefaultSlackConfig returns default configuration from environment.
func DefaultSlackConfig() SlackConfig {
	return SlackConfig{
		WebhookURL:       os.Getenv("SLACK_WEBHOOK_URL"),
		Channel:          os.Getenv("SLACK_CHANNEL"),
		Username:         "LLMux",
		IconEmoji:        ":robot_face:",
		AlertOnErrors:    true,
		AlertOnFallbacks: true,
		AlertOnBudget:    true,
		MinErrorInterval: time.Minute,
		ErrorThreshold:   1,
	}
}

// SlackCallback implements Callback for Slack alerting.
type SlackCallback struct {
	config     SlackConfig
	client     *http.Client
	lastAlert  time.Time
	errorCount int
	mu         sync.Mutex
}

// slackMessage represents a Slack message payload.
type slackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	Text        string            `json:"text,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

// slackAttachment represents a Slack message attachment.
type slackAttachment struct {
	Color      string       `json:"color,omitempty"`
	Title      string       `json:"title,omitempty"`
	Text       string       `json:"text,omitempty"`
	Fields     []slackField `json:"fields,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	Timestamp  int64        `json:"ts,omitempty"`
	MarkdownIn []string     `json:"mrkdwn_in,omitempty"`
}

// slackField represents a field in a Slack attachment.
type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackCallback creates a new Slack callback.
func NewSlackCallback(cfg SlackConfig) (*SlackCallback, error) {
	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("slack: webhook_url is required")
	}

	return &SlackCallback{
		config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Name returns the callback name.
func (s *SlackCallback) Name() string {
	return "slack"
}

// LogPreAPICall is a no-op for Slack.
func (s *SlackCallback) LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	return nil
}

// LogPostAPICall is a no-op for Slack.
func (s *SlackCallback) LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	return nil
}

// LogStreamEvent is a no-op for Slack.
func (s *SlackCallback) LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) error {
	return nil
}

// LogSuccessEvent is a no-op for Slack (we only alert on errors).
func (s *SlackCallback) LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error {
	// Reset error count on success
	s.mu.Lock()
	s.errorCount = 0
	s.mu.Unlock()
	return nil
}

// LogFailureEvent sends an alert for failed requests.
func (s *SlackCallback) LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error {
	if !s.config.AlertOnErrors {
		return nil
	}

	// Rate limiting
	s.mu.Lock()
	s.errorCount++
	if s.errorCount < s.config.ErrorThreshold {
		s.mu.Unlock()
		return nil
	}
	if time.Since(s.lastAlert) < s.config.MinErrorInterval {
		s.mu.Unlock()
		return nil
	}
	s.lastAlert = time.Now()
	errorCount := s.errorCount
	s.errorCount = 0
	s.mu.Unlock()

	// Build error message
	errorMsg := "Unknown error"
	if err != nil {
		errorMsg = err.Error()
	} else if payload.ErrorStr != nil {
		errorMsg = *payload.ErrorStr
	}

	msg := s.buildErrorMessage(payload, errorMsg, errorCount)
	return s.send(ctx, msg)
}

// LogFallbackEvent sends an alert for fallback events.
func (s *SlackCallback) LogFallbackEvent(ctx context.Context, originalModel, fallbackModel string, err error, success bool) error {
	if !s.config.AlertOnFallbacks {
		return nil
	}

	msg := s.buildFallbackMessage(originalModel, fallbackModel, err, success)
	return s.send(ctx, msg)
}

// Shutdown is a no-op for Slack.
func (s *SlackCallback) Shutdown(ctx context.Context) error {
	return nil
}

// SendBudgetAlert sends a budget warning alert.
func (s *SlackCallback) SendBudgetAlert(ctx context.Context, entityType, entityID string, remaining, maxBudget float64, percentUsed float64) error {
	if !s.config.AlertOnBudget {
		return nil
	}

	msg := s.buildBudgetMessage(entityType, entityID, remaining, maxBudget, percentUsed)
	return s.send(ctx, msg)
}

// buildErrorMessage builds a Slack message for an error.
func (s *SlackCallback) buildErrorMessage(payload *StandardLoggingPayload, errorMsg string, errorCount int) slackMessage {
	fields := []slackField{
		{Title: "Model", Value: payload.Model, Short: true},
		{Title: "Provider", Value: payload.APIProvider, Short: true},
		{Title: "Request ID", Value: payload.RequestID, Short: true},
		{Title: "Call Type", Value: string(payload.CallType), Short: true},
	}

	if payload.Team != nil {
		fields = append(fields, slackField{Title: "Team", Value: *payload.Team, Short: true})
	}
	if payload.User != nil {
		fields = append(fields, slackField{Title: "User", Value: *payload.User, Short: true})
	}

	// Truncate error message if too long
	if len(errorMsg) > 500 {
		errorMsg = errorMsg[:500] + "..."
	}

	title := ":x: LLM Request Failed"
	if errorCount > 1 {
		title = fmt.Sprintf(":x: LLM Request Failed (%d errors)", errorCount)
	}

	return slackMessage{
		Channel:   s.config.Channel,
		Username:  s.config.Username,
		IconEmoji: s.config.IconEmoji,
		Attachments: []slackAttachment{
			{
				Color:      "danger",
				Title:      title,
				Text:       fmt.Sprintf("```%s```", errorMsg),
				Fields:     fields,
				Footer:     "LLMux Alert",
				Timestamp:  time.Now().Unix(),
				MarkdownIn: []string{"text"},
			},
		},
	}
}

// buildFallbackMessage builds a Slack message for a fallback event.
func (s *SlackCallback) buildFallbackMessage(originalModel, fallbackModel string, err error, success bool) slackMessage {
	var color, title, text string

	if success {
		color = "warning"
		title = ":warning: Fallback Triggered (Success)"
		text = fmt.Sprintf("Request to `%s` failed, successfully fell back to `%s`", originalModel, fallbackModel)
	} else {
		color = "danger"
		title = ":x: Fallback Failed"
		text = fmt.Sprintf("Request to `%s` failed, fallback to `%s` also failed", originalModel, fallbackModel)
	}

	fields := []slackField{
		{Title: "Original Model", Value: originalModel, Short: true},
		{Title: "Fallback Model", Value: fallbackModel, Short: true},
	}

	if err != nil {
		errMsg := err.Error()
		if len(errMsg) > 200 {
			errMsg = errMsg[:200] + "..."
		}
		fields = append(fields, slackField{Title: "Error", Value: errMsg, Short: false})
	}

	return slackMessage{
		Channel:   s.config.Channel,
		Username:  s.config.Username,
		IconEmoji: s.config.IconEmoji,
		Attachments: []slackAttachment{
			{
				Color:      color,
				Title:      title,
				Text:       text,
				Fields:     fields,
				Footer:     "LLMux Alert",
				Timestamp:  time.Now().Unix(),
				MarkdownIn: []string{"text"},
			},
		},
	}
}

// buildBudgetMessage builds a Slack message for a budget warning.
func (s *SlackCallback) buildBudgetMessage(entityType, entityID string, remaining, maxBudget, percentUsed float64) slackMessage {
	var color string
	if percentUsed >= 100 {
		color = "danger"
	} else if percentUsed >= 90 {
		color = "warning"
	} else {
		color = "good"
	}

	caser := cases.Title(language.English)
	title := fmt.Sprintf(":moneybag: Budget Alert: %s", caser.String(entityType))
	text := fmt.Sprintf("%s `%s` has used %.1f%% of budget", caser.String(entityType), entityID, percentUsed)

	fields := []slackField{
		{Title: "Remaining", Value: fmt.Sprintf("$%.2f", remaining), Short: true},
		{Title: "Max Budget", Value: fmt.Sprintf("$%.2f", maxBudget), Short: true},
		{Title: "Usage", Value: fmt.Sprintf("%.1f%%", percentUsed), Short: true},
	}

	return slackMessage{
		Channel:   s.config.Channel,
		Username:  s.config.Username,
		IconEmoji: s.config.IconEmoji,
		Attachments: []slackAttachment{
			{
				Color:      color,
				Title:      title,
				Text:       text,
				Fields:     fields,
				Footer:     "LLMux Alert",
				Timestamp:  time.Now().Unix(),
				MarkdownIn: []string{"text"},
			},
		},
	}
}

// send sends a message to Slack.
func (s *SlackCallback) send(ctx context.Context, msg slackMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("slack: failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: failed to send message: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack: webhook returned status %d", resp.StatusCode)
	}

	return nil
}
