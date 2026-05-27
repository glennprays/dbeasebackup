package notifier

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

func testLogger(t *testing.T) *log.Logger {
	t.Helper()
	l, err := log.New(log.Config{
		Service: "test",
		Env:     "development",
		Level:   log.ErrorLevel,
		Output:  log.OutputStdout,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	return l
}

func testContext() context.Context {
	return traceid.NewContext(context.Background(), "test-trace-id")
}

func TestNewWebhookNotifier_EmptyURL(t *testing.T) {
	n := NewWebhookNotifier(WebhookConfig{URL: ""}, testLogger(t))
	if n != nil {
		t.Error("expected nil for empty URL")
	}
}

func TestNewWebhookNotifier_DefaultTimeout(t *testing.T) {
	n := NewWebhookNotifier(WebhookConfig{URL: "http://example.com"}, testLogger(t))
	if n == nil {
		t.Fatal("expected non-nil notifier")
	}
	if n.client.Timeout != 10*time.Second {
		t.Errorf("expected 10s default timeout, got %v", n.client.Timeout)
	}
}

func TestNewWebhookNotifier_CustomTimeout(t *testing.T) {
	n := NewWebhookNotifier(WebhookConfig{URL: "http://example.com", Timeout: 5 * time.Second}, testLogger(t))
	if n == nil {
		t.Fatal("expected non-nil notifier")
	}
	if n.client.Timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", n.client.Timeout)
	}
}

func TestWebhookNotifier_Notify_Success(t *testing.T) {
	var receivedBody []byte
	var receivedHeaders http.Header
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedHeaders = r.Header
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewWebhookNotifier(WebhookConfig{URL: server.URL, Secret: "test-secret"}, testLogger(t))

	event := Event{
		EventType:   EventBackup,
		Status:      StatusSuccess,
		Filename:    "backup_2026-05-28.tar",
		FileSize:    1024,
		Duration:    "1m30s",
		Provider:    "postgres-backup",
		StorageType: "s3",
		Timestamp:   time.Date(2026, 5, 28, 2, 0, 0, 0, time.UTC),
		TraceID:     "test-trace-id",
	}

	n.Notify(testContext(), event)

	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json content type, got %s", receivedHeaders.Get("Content-Type"))
	}
	if receivedHeaders.Get("User-Agent") != "DBEaseBackup-Webhook/1.0" {
		t.Errorf("expected DBEaseBackup-Webhook/1.0 user agent, got %s", receivedHeaders.Get("User-Agent"))
	}

	var received Event
	if err := json.Unmarshal(receivedBody, &received); err != nil {
		t.Fatalf("failed to unmarshal received body: %v", err)
	}
	if received.EventType != EventBackup {
		t.Errorf("expected event_type=backup, got %s", received.EventType)
	}
	if received.Status != StatusSuccess {
		t.Errorf("expected status=success, got %s", received.Status)
	}
	if received.Filename != "backup_2026-05-28.tar" {
		t.Errorf("expected filename backup_2026-05-28.tar, got %s", received.Filename)
	}
	if received.FileSize != 1024 {
		t.Errorf("expected file_size_bytes=1024, got %d", received.FileSize)
	}
	if received.Error != "" {
		t.Errorf("expected empty error for success event, got %s", received.Error)
	}
}

func TestWebhookNotifier_Notify_HMACSignature(t *testing.T) {
	secret := "my-webhook-secret"
	var receivedBody []byte
	var receivedSig string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature-256")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewWebhookNotifier(WebhookConfig{URL: server.URL, Secret: secret}, testLogger(t))

	event := Event{
		EventType: EventBackup,
		Status:    StatusSuccess,
		Duration:  "1s",
		Provider:  "postgres-backup",
		Timestamp: time.Date(2026, 5, 28, 2, 0, 0, 0, time.UTC),
		TraceID:   "test-trace",
	}

	n.Notify(testContext(), event)

	if !strings.HasPrefix(receivedSig, "sha256=") {
		t.Fatalf("expected signature to start with sha256=, got %s", receivedSig)
	}

	receivedHex := strings.TrimPrefix(receivedSig, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(receivedBody)
	expectedHex := hex.EncodeToString(mac.Sum(nil))

	if receivedHex != expectedHex {
		t.Errorf("HMAC mismatch: got %s, expected %s", receivedHex, expectedHex)
	}
}

func TestWebhookNotifier_Notify_NoSecret(t *testing.T) {
	var receivedSig string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature-256")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewWebhookNotifier(WebhookConfig{URL: server.URL}, testLogger(t))

	n.Notify(testContext(), Event{
		EventType: EventBackup,
		Status:    StatusSuccess,
		Duration:  "1s",
		Provider:  "test",
		Timestamp: time.Now(),
		TraceID:   "t",
	})

	if receivedSig != "" {
		t.Errorf("expected no X-Signature-256 header when secret is empty, got %s", receivedSig)
	}
}

func TestWebhookNotifier_Notify_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	n := NewWebhookNotifier(WebhookConfig{URL: server.URL}, testLogger(t))

	// Should not panic
	n.Notify(testContext(), Event{
		EventType: EventBackup,
		Status:    StatusFailure,
		Duration:  "0s",
		Provider:  "test",
		Timestamp: time.Now(),
		TraceID:   "t",
	})
}

func TestWebhookNotifier_Notify_ConnectionRefused(t *testing.T) {
	n := NewWebhookNotifier(WebhookConfig{
		URL:     "http://127.0.0.1:1",
		Timeout: 1 * time.Second,
	}, testLogger(t))

	// Should not panic
	n.Notify(testContext(), Event{
		EventType: EventBackup,
		Status:    StatusFailure,
		Duration:  "0s",
		Provider:  "test",
		Timestamp: time.Now(),
		TraceID:   "t",
	})
}

func TestWebhookNotifier_Notify_SuccessOmitsErrorField(t *testing.T) {
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewWebhookNotifier(WebhookConfig{URL: server.URL}, testLogger(t))

	n.Notify(testContext(), Event{
		EventType: EventBackup,
		Status:    StatusSuccess,
		Filename:  "backup.tar",
		Duration:  "1s",
		Provider:  "test",
		Timestamp: time.Now(),
		TraceID:   "t",
	})

	var raw map[string]any
	if err := json.Unmarshal(receivedBody, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, exists := raw["error"]; exists {
		t.Error("expected error field to be omitted for success event")
	}
}

func TestComputeHMAC(t *testing.T) {
	message := []byte("hello world")
	key := []byte("secret")

	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	expected := hex.EncodeToString(mac.Sum(nil))

	got := computeHMAC(message, key)
	if got != expected {
		t.Errorf("computeHMAC mismatch: got %s, expected %s", got, expected)
	}
}
