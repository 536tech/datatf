package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	Endpoint = "https://telemetry.datatf.io/v1/events"
	Notice   = "https://github.com/536tech/datatf/blob/main/docs/telemetry.md"
	Timeout  = 250 * time.Millisecond
)

// Send attempts one event after consent. Delivery never changes the command result.
func Send(ctx context.Context, run Run) {
	_ = send(ctx, run, Endpoint)
}

func send(ctx context.Context, run Run, endpoint string) error {
	payload, err := Payload(run)
	if err != nil {
		return err
	}
	if len(payload) > 2048 {
		return fmt.Errorf("telemetry payload exceeds 2048 bytes")
	}
	// A canceled command can report cancellation within the same short delivery budget.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "datatf")
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport, Timeout: Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("telemetry collector returned HTTP %d", response.StatusCode)
	}
	return nil
}
