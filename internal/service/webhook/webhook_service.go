package webhook

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebhookService handles sending webhook notifications
type WebhookService struct {
	client *http.Client
}

// NewWebhookService creates a new WebhookService
func NewWebhookService() *WebhookService {
	return &WebhookService{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// SendToWebhook sends a payload to the specified webhook URL
func (ws *WebhookService) SendToWebhook(payload []byte, webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL cannot be empty")
	}

	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https scheme")
	}

	// Consider additional checks: block localhost, private IPs, link-local addresses

	req, err := http.NewRequest("POST", parsedURL.String(), bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := ws.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-success status: %d", resp.StatusCode)
	}

	return nil
}
