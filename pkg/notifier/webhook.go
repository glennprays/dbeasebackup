package notifier

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

type WebhookConfig struct {
	URL     string
	Secret  string
	Timeout time.Duration
}

type WebhookNotifier struct {
	client *http.Client
	cfg    WebhookConfig
	logger *log.Logger
}

// NewWebhookNotifier creates a new webhook notifier. Returns nil if URL is empty.
func NewWebhookNotifier(cfg WebhookConfig, logger *log.Logger) *WebhookNotifier {
	if cfg.URL == "" {
		return nil
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &WebhookNotifier{
		client: &http.Client{Timeout: cfg.Timeout},
		cfg:    cfg,
		logger: logger,
	}
}

func (w *WebhookNotifier) Notify(ctx context.Context, event Event) {
	traceID := traceid.FromContext(ctx)

	payload, err := json.Marshal(event)
	if err != nil {
		w.logger.Warn(traceID, "Failed to marshal webhook payload", nil,
			log.Error(err),
			log.String("component", "webhook-notifier"),
		)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.URL, bytes.NewReader(payload))
	if err != nil {
		w.logger.Warn(traceID, "Failed to create webhook request", nil,
			log.Error(err),
			log.String("component", "webhook-notifier"),
		)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DBEaseBackup-Webhook/1.0")

	if w.cfg.Secret != "" {
		sig := computeHMAC(payload, []byte(w.cfg.Secret))
		req.Header.Set("X-Signature-256", "sha256="+sig)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.Warn(traceID, "Webhook delivery failed", nil,
			log.Error(err),
			log.String("url", w.cfg.URL),
			log.String("component", "webhook-notifier"),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		w.logger.Warn(traceID, "Webhook endpoint returned non-success status", nil,
			log.Int("status_code", resp.StatusCode),
			log.String("url", w.cfg.URL),
			log.String("component", "webhook-notifier"),
		)
		return
	}

	w.logger.Info(traceID, "Webhook notification sent", nil,
		log.Int("status_code", resp.StatusCode),
		log.String("event_type", string(event.EventType)),
		log.String("event_status", string(event.Status)),
		log.String("component", "webhook-notifier"),
	)
}

func computeHMAC(message, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil))
}

var _ Notifier = (*WebhookNotifier)(nil)
