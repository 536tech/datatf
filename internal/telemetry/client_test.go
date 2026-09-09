package telemetry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSendUsesOneBoundedJSONRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assertRequestMetadata(t, r)
		body, err := io.ReadAll(r.Body)
		want, _ := Payload(sampleRun())
		if err != nil || string(body) != string(want) {
			t.Errorf("request differs from preview: %s %v", body, err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	if err := send(context.Background(), sampleRun(), server.URL); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("sent %d requests", calls.Load())
	}
}

func assertRequestMetadata(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
		t.Errorf("unexpected request: %s %v", r.Method, r.Header)
	}
	if r.Header.Get("Authorization") != "" || r.URL.RawQuery != "" {
		t.Error("unexpected credentials or query parameters")
	}
}

func TestSendDoesNotRetryErrorsOrFollowRedirects(t *testing.T) {
	for _, status := range []int{400, 429, 500, 503, 307} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/redirected")
				w.WriteHeader(status)
			}))
			defer server.Close()
			if err := send(context.Background(), sampleRun(), server.URL); err == nil {
				t.Fatal("expected rejected delivery")
			}
			if calls.Load() != 1 {
				t.Fatalf("sent %d requests", calls.Load())
			}
		})
	}
}

func TestSendTimeoutAndCanceledCommand(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer server.Close()
	defer close(release)
	start := time.Now()
	if err := send(context.Background(), sampleRun(), server.URL); err == nil {
		t.Fatal("expected timeout")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("delivery exceeded the bounded timeout: %v", elapsed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer fast.Close()
	if err := send(ctx, sampleRun(), fast.URL); err != nil {
		t.Fatalf("canceled command cannot report its outcome: %v", err)
	}
}

func TestSendRejectsTLSFailureAndUnsupportedCommand(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("untrusted TLS reached the collector")
	}))
	defer server.Close()
	if err := send(context.Background(), sampleRun(), server.URL); err == nil {
		t.Fatal("TLS verification was bypassed")
	}
	run := sampleRun()
	run.Command = "auth"
	if err := send(context.Background(), run, "http://invalid.invalid"); err == nil {
		t.Fatal("unsupported commands must not send")
	}
}
