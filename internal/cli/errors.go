package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/536tech/datatf/internal/diagnostic"
	"github.com/536tech/datatf/internal/inventory"
)

type commandError struct {
	error
	diagnostic.Details
}

func (e *commandError) Unwrap() error { return e.error }

func withHint(err error, code, hint string) error {
	return &commandError{error: err, Details: diagnostic.Details{Code: code, Hint: hint}}
}

func (rc *runtime) writeError(cmd *cobra.Command, err error) int {
	details, exit := diagnostic.Describe(err), exitErr
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		details = commandErr.Details
	}
	if errors.Is(err, errUsage) {
		exit = exitUsage
		details = diagnostic.Details{Code: "invalid_usage", Hint: "Run " + cmd.CommandPath() + " --help."}
	}
	if rc.usage != nil {
		rc.usage.ErrorCode = details.Code
	}
	if rc.g.asJSON {
		payload := struct {
			Error any `json:"error"`
		}{Error: struct {
			diagnostic.Details
			Message string `json:"message"`
		}{details, err.Error()}}
		if writeErr := json.NewEncoder(rc.stderr).Encode(payload); writeErr != nil {
			return exitErr
		}
		return exit
	}
	_, _ = fmt.Fprintf(rc.stderr, "error [%s]: %s\nhint: %s\n",
		details.Code, terminalText(err.Error()), details.Hint)
	return exit
}

func (rc *runtime) partialError(operation string, issues []inventory.Issue) error {
	if len(issues) == 0 {
		return nil
	}
	if !rc.g.asJSON {
		for _, issue := range issues {
			rc.out.Errorf("issue: %s | %s | %s | %s\n", terminalText(issue.Area),
				terminalText(issue.ObjectName), terminalText(issue.Operation), terminalText(issue.Message))
			if issue.Hint != "" {
				rc.out.Errorf("hint: %s\n", issue.Hint)
			}
		}
	}
	return withHint(fmt.Errorf("%s is partial (%d issues)", operation, len(issues)), "partial_result",
		"Fix the reported issues before export. Use datatf inventory --help to check the selection. "+
			"Keep the same profile and host.")
}

func workspaceError(err error) error {
	if diagnostic.Describe(err).Code != "operation_failed" {
		return err
	}
	return withHint(err, "workspace_error", "Check the connection with datatf auth status. "+
		"Keep the same profile and host. If the CLI is available, use databricks auth describe.")
}

func outputError(err error) error {
	return withHint(err, "output_error",
		"Check the destination and disk access. Use --out with a new directory. Keep existing files.")
}

func terminalText(value string) string {
	var text strings.Builder
	for _, char := range value {
		if unicode.IsControl(char) {
			quoted := strconv.QuoteRune(char)
			text.WriteString(quoted[1 : len(quoted)-1])
		} else {
			text.WriteRune(char)
		}
	}
	return text.String()
}
