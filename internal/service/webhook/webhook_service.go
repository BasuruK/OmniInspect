package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Reserved IP ranges to block for security
var reservedCIDRStrings = []string{
	"127.0.0.0/8",     // Loopback
	"::1/128",         // Loopback IPv6
	"10.0.0.0/8",      // Private RFC1918
	"172.16.0.0/12",   // Private RFC1918
	"192.168.0.0/16",  // Private RFC1918
	"169.254.0.0/16",  // Link-local
	"fe80::/10",       // Link-local IPv6
	"fc00::/7",        // Unique local IPv6
	"224.0.0.0/4",     // Multicast
	"ff00::/8",        // Multicast IPv6
	"0.0.0.0/8",       // Current network
	"100.64.0.0/10",   // Carrier-grade NAT
	"192.0.0.0/24",    // IETF Protocol
	"192.0.2.0/24",    // TEST-NET-1
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
}

// Known metadata endpoints to block
var metadataEndpoints = []string{
	"169.254.169.254", // AWS, GCP, Azure metadata
	"metadata.google.internal",
}

// Pre-parsed reserved IP networks for efficiency
var reservedIPNets []*net.IPNet

func init() {
	// Pre-parse all CIDRs at package load time
	reservedIPNets = make([]*net.IPNet, 0, len(reservedCIDRStrings))
	for _, cidr := range reservedCIDRStrings {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		reservedIPNets = append(reservedIPNets, network)
	}
}

func isReservedIP(ip net.IP) bool {
	for _, network := range reservedIPNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func isHostnameBlocked(host string) bool {
	hostLower := strings.ToLower(host)
	for _, meta := range metadataEndpoints {
		if strings.Contains(hostLower, meta) {
			return true
		}
	}
	return false
}

// WebhookPayload represents the JSON envelope for webhook messages
type WebhookPayload struct {
	Message   string `json:"message"`
	LogLevel  string `json:"log_level,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// WebhookMetadata holds optional metadata for webhook messages
type WebhookMetadata struct {
	LogLevel  string
	Timestamp string
}

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

// SendToWebhook sends a payload to the specified webhook URL as a JSON envelope
func (ws *WebhookService) SendToWebhook(payload []byte, webhookURL string, meta ...WebhookMetadata) error {
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

	// Security: validate host is not localhost or reserved IP
	host := parsedURL.Hostname()
	if host == "" {
		return fmt.Errorf("webhook URL must have a valid host")
	}

	// Block localhost aliases
	if strings.ToLower(host) == "localhost" || host == "0.0.0.0" || host == "::" {
		return fmt.Errorf("webhook URL cannot point to localhost")
	}

	// Block known metadata endpoints
	if isHostnameBlocked(host) {
		return fmt.Errorf("webhook URL cannot point to cloud metadata endpoints")
	}

	// Resolve and validate IP addresses
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve webhook hostname: %w", err)
	}
	for _, ip := range ips {
		if isReservedIP(ip) {
			return fmt.Errorf("webhook URL cannot point to reserved/private IP address: %s", ip.String())
		}
	}

	// Wrap payload in JSON envelope with optional metadata
	envelope := WebhookPayload{
		Message: string(payload),
	}
	if len(meta) > 0 {
		envelope.LogLevel = meta[0].LogLevel
		envelope.Timestamp = meta[0].Timestamp
	}

	jsonBody, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequest("POST", parsedURL.String(), bytes.NewBuffer(jsonBody))
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
