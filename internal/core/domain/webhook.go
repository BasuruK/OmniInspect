package domain

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ==========================================
// Webhook Configuration Entity
// ==========================================

// WebhookConfig represents a webhook configuration stored in BoltDB
type WebhookConfig struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewWebhookConfig creates a new WebhookConfig with URL validation
func NewWebhookConfig(id string, urlStr string, enabled bool) (*WebhookConfig, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("URL must use http or https scheme")
	}

	if parsedURL.Host == "" {
		return nil, fmt.Errorf("URL must have a valid host")
	}

	now := time.Now()
	return &WebhookConfig{
		ID:        id,
		URL:       urlStr,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ==========================================
// Business Methods
// ==========================================

// IsConfigured returns true if webhook URL is set and enabled
func (w *WebhookConfig) IsConfigured() bool {
	return w != nil && w.URL != "" && w.Enabled
}

// ==========================================
// Constants
// ==========================================

const (
	// DefaultWebhookID is the single webhook ID used
	DefaultWebhookID = "default"
)
