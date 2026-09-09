package diagnostic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/databricks/databricks-sdk-go/apierr"
	"github.com/databricks/databricks-sdk-go/config"
)

func TestDescribeTypedFailures(t *testing.T) {
	tests := []struct {
		err  error
		code string
	}{
		{context.Canceled, "canceled"},
		{context.DeadlineExceeded, "timeout"},
		{config.ErrCannotConfigureDefault, "configuration_error"},
		{config.ErrNoHostConfigured, "configuration_error"},
		{&net.DNSError{Err: "no such host", Name: "example.invalid"}, "connection_failed"},
		{&apierr.APIError{StatusCode: 401}, "authentication_failed"},
		{&apierr.APIError{StatusCode: 403}, "permission_denied"},
		{&apierr.APIError{StatusCode: 404}, "not_found"},
		{&apierr.APIError{StatusCode: 429}, "resource_exhausted"},
		{&apierr.APIError{StatusCode: 503}, "service_error"},
		{&apierr.APIError{StatusCode: 400, ErrorCode: "NEW_ERROR"}, "api_error"},
		{&apierr.APIError{StatusCode: 400, ErrorCode: "UNAUTHENTICATED"}, "authentication_failed"},
		{&apierr.APIError{StatusCode: 400, ErrorCode: "PERMISSION_DENIED"}, "permission_denied"},
		{&apierr.APIError{StatusCode: 400, ErrorCode: "RESOURCE_EXHAUSTED"}, "resource_exhausted"},
		{errors.New("PERMISSION_DENIED 401 token expired"), "operation_failed"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := Describe(fmt.Errorf("read workspace: %w", tt.err))
			if got.Code != tt.code || got.Hint == "" {
				t.Fatalf("Describe(%v) = %+v", tt.err, got)
			}
		})
	}
}

func TestDescribePreservesAPIEvidence(t *testing.T) {
	err := &apierr.APIError{StatusCode: 403, ErrorCode: "PERMISSION_DENIED", Message: "untrusted"}
	got := Describe(err)
	if got.APICode != err.ErrorCode || got.HTTPStatus != err.StatusCode {
		t.Fatalf("lost API evidence: %+v", got)
	}
	for _, unsafe := range []string{"untrusted", "--allow-partial", "--sensitive", "auth login"} {
		if strings.Contains(got.Hint, unsafe) {
			t.Fatalf("unsafe hint: %s", got.Hint)
		}
	}
}
