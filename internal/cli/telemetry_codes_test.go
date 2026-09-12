package cli

import (
	"encoding/json"
	"testing"

	"github.com/536tech/datatf/internal/telemetry"
)

// Every error code the CLI attaches with withHint, plus every diagnostic category, must be
// an allowlisted telemetry category. Otherwise the collector records it as "other".
func TestTelemetryAllowsEveryCLIErrorCode(t *testing.T) {
	for _, code := range []string{
		"invalid_usage", "configuration_error", "authentication_failed", "permission_denied",
		"not_found", "resource_exhausted", "service_error", "api_error", "connection_failed",
		"operation_failed", "timeout", "canceled", "partial_result", "output_error",
		"workspace_error", "render_error",
	} {
		payload, err := telemetry.Payload(telemetry.Run{
			Version: "1.0.0", OS: "linux", Arch: "amd64", Command: "export", Scope: "workspace",
			Outcome: "error", ErrorCode: code,
		})
		if err != nil {
			t.Fatal(err)
		}
		var event struct {
			ErrorCode string `json:"error_code"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatal(err)
		}
		if event.ErrorCode != code {
			t.Errorf("%s recorded as %s", code, event.ErrorCode)
		}
	}
}
