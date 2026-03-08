package domain

import (
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

// NewWebhookConfig creates a new WebhookConfig
func NewWebhookConfig(id string, url string, enabled bool) *WebhookConfig {
	now := time.Now()
	return &WebhookConfig{
		ID:        id,
		URL:       url,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
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
