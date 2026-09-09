// Package diagnostic gives read failures stable codes and safe recovery hints.
package diagnostic

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/databricks/databricks-sdk-go/apierr"
	"github.com/databricks/databricks-sdk-go/config"
)

// Details supplements an error without changing its message or cause.
type Details struct {
	Code       string `json:"code,omitempty"`
	Hint       string `json:"hint,omitempty"`
	APICode    string `json:"api_code,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
}

// Describe uses typed causes, not message text, to select recovery guidance.
func Describe(err error) Details {
	var api *apierr.APIError
	var network net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return Details{Code: "canceled",
			Hint: "The command was canceled. Review any output before another run."}
	case errors.Is(err, context.DeadlineExceeded):
		return Details{Code: "timeout",
			Hint: "Check the workspace connection and the caller's time limit."}
	case errors.Is(err, config.ErrCannotConfigureDefault), errors.Is(err, config.ErrNoHostConfigured):
		return Details{Code: "configuration_error", Hint: ConfigurationHint}
	case errors.As(err, &api):
		details := apiAdvice(api)
		details.APICode, details.HTTPStatus = api.ErrorCode, api.StatusCode
		return details
	case errors.As(err, &network):
		return Details{Code: "connection_failed",
			Hint: "Check the workspace URL, VPN, proxy, DNS, and TLS access. " +
				"Keep certificate checks enabled."}
	default:
		return Details{Code: "operation_failed",
			Hint: "Check the reported operation. Run datatf version before you report an issue."}
	}
}

// ConfigurationHint avoids a login command that could change M2M or CI authentication.
const ConfigurationHint = "Check the selected host and authentication settings. " +
	"If you use profiles, run databricks auth profiles."

func apiAdvice(api *apierr.APIError) Details {
	// Prefer recognized API codes; gateways and legacy endpoints need an HTTP fallback.
	status := api.StatusCode
	switch api.ErrorCode {
	case "UNAUTHENTICATED":
		status = http.StatusUnauthorized
	case "PERMISSION_DENIED":
		status = http.StatusForbidden
	case "RESOURCE_EXHAUSTED":
		status = http.StatusTooManyRequests
	}
	return statusAdvice(status)
}

func statusAdvice(status int) Details {
	switch {
	case status == http.StatusUnauthorized:
		return Details{Code: "authentication_failed",
			Hint: "Check credentials and the workspace URL. Use databricks auth describe " +
				"with the same profile and host if the CLI is available."}
	case status == http.StatusForbidden:
		return Details{Code: "permission_denied",
			Hint: "Confirm workspace access and the failed operation's permissions with its owner. " +
				"See https://github.com/536tech/datatf/blob/main/docs/permissions.md."}
	case status == http.StatusNotFound:
		return Details{Code: "not_found",
			Hint: "Confirm the workspace and visible object name with datatf inventory --json. " +
				"Keep the same profile, host, and resource selection."}
	case status == http.StatusTooManyRequests:
		return Details{Code: "resource_exhausted",
			Hint: "Check the reported rate limit or quota before another attempt. " +
				"The SDK already retries some failures."}
	case status >= 500:
		return Details{Code: "service_error",
			Hint: "Check the Databricks service and gateway status. " +
				"The SDK already retries transient failures."}
	default:
		return Details{Code: "api_error",
			Hint: "Check the reported API code and operation. " +
				"Run datatf version before you report an issue."}
	}
}
