// Package cli wires the datatf commands.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/databricks/databricks-sdk-go"
	"github.com/spf13/cobra"

	"github.com/536tech/datatf/internal/diagnostic"
	"github.com/536tech/datatf/internal/output"
	"github.com/536tech/datatf/internal/telemetry"
)

const (
	exitOK    = 0
	exitErr   = 1
	exitUsage = 2
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

type globals struct {
	profile     string
	host        string
	asJSON      bool
	plain       bool
	quiet       bool
	noColor     bool
	showVersion bool
}

type runtime struct {
	ctx          context.Context
	stdin        io.Reader
	stdout       io.Writer
	stderr       io.Writer
	g            *globals
	out          *output.Formatter
	usage        *telemetry.Run
	usageStarted time.Time
}

// Execute runs the CLI and returns the process exit code.
func Execute(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rc := &runtime{ctx: ctx, stdin: stdin, stdout: stdout, stderr: stderr, g: &globals{}}

	cmd := newRootCommand(rc)
	cmd.SetArgs(args)
	cmd.SetIn(stdin)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	executed, err := cmd.ExecuteContextC(ctx)
	code := exitOK
	if err != nil {
		code = rc.writeError(executed, err)
	}
	rc.finishUsage(err != nil)
	if err == nil || len(args) == 0 {
		rc.updateNotice(executed)
	}
	return code
}

func newRootCommand(rc *runtime) *cobra.Command {
	root := &cobra.Command{
		Use:   "datatf",
		Args:  usageArgs(cobra.NoArgs),
		Short: "Export supported Azure Databricks platform configuration to Terraform",
		Long: `datatf exports supported platform configuration from an existing Azure
Databricks workspace into an opinionated Terraform root. It writes Terraform
variable values and import blocks for the datatf module contract.

The command is read-only against Databricks and never reads secret values.

Authentication uses the Databricks SDK unified auth: --profile/--host, the
DATABRICKS_* environment variables, ~/.databrickscfg, Azure CLI, or OAuth M2M.

Use auth status to check the connection. Use inventory --json to inspect
visible resources without local files. Neither proves full resource visibility.`,
		Example: `  datatf auth status --profile analytics
  datatf --json inventory --profile analytics --resources catalogs
  datatf export --profile analytics --scaffold`,
		SuggestionsMinimumDistance: 2,
		SilenceUsage:               true,
		SilenceErrors:              true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rc.g.showVersion {
				return rc.writeVersion()
			}
			return commandRequired(cmd, rc)
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if rc.g.asJSON && rc.g.plain {
				return fmt.Errorf("%w: choose only one of --json or --plain", errUsage)
			}
			rc.out = output.New(rc.stdout, rc.stderr, rc.g.asJSON, rc.g.plain, false, rc.g.noColor)
			rc.beginUsage(cmd.Name())
			return nil
		},
	}
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return fmt.Errorf("%w: %v", errUsage, err)
	})

	flags := root.PersistentFlags()
	flags.StringVarP(&rc.g.profile, "profile", "p", "", "Databricks CLI profile (DATABRICKS_CONFIG_PROFILE)")
	flags.StringVar(&rc.g.host, "host", "", "Databricks workspace URL (DATABRICKS_HOST)")
	flags.BoolVar(&rc.g.asJSON, "json", false,
		"emit JSON results to stdout and errors to stderr; no progress")
	flags.BoolVar(&rc.g.plain, "plain", false, "emit stable plain text where available")
	flags.BoolVarP(&rc.g.quiet, "quiet", "q", false, "suppress progress and update notices on stderr")
	flags.BoolVar(&rc.g.noColor, "no-color", false, "disable color")
	flags.BoolVar(&rc.g.showVersion, "version", false, "print version and exit")

	root.AddCommand(newInventoryCommand(rc))
	root.AddCommand(newExportCommand(rc))
	root.AddCommand(newAuthCommand(rc))
	root.AddCommand(newVersionCommand(rc))
	root.AddCommand(newTelemetryCommand(rc))
	root.AddCommand(newCompletionCommand(root))

	return root
}

func newVersionCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Args:  usageArgs(cobra.NoArgs),
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return rc.writeVersion()
		},
	}
}

func (rc *runtime) writeVersion() error {
	payload := map[string]string{"version": version, "commit": commit, "date": date}
	if rc.out.IsJSON() {
		return rc.out.JSON(payload)
	}
	if rc.out.IsPlain() {
		rc.out.Printf("%s\n", version)
		return nil
	}
	rc.out.Printf("datatf version %s\n", version)
	rc.out.Printf("commit: %s\n", commit)
	rc.out.Printf("built:  %s\n", date)
	return nil
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletion(cmd.OutOrStdout())
			default:
				return fmt.Errorf("%w: unsupported shell %q", errUsage, args[0])
			}
		},
	}
}

var errUsage = errors.New("invalid usage")

// client builds the SDK workspace client from unified auth plus --profile/--host.
func (rc *runtime) client() (*databricks.WorkspaceClient, error) {
	ws, err := databricks.NewWorkspaceClient(&databricks.Config{Profile: rc.g.profile, Host: rc.g.host})
	if err != nil {
		return nil, withHint(fmt.Errorf("create Databricks client: %w", err),
			"configuration_error", diagnostic.ConfigurationHint)
	}
	return ws, nil
}

// now is replaceable in tests.
var now = time.Now

func usageArgs(fn cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := fn(cmd, args); err != nil {
			if len(args) > 0 {
				if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
					err = fmt.Errorf("%w; did you mean %s?", err, strings.Join(suggestions, ", "))
				}
			}
			return fmt.Errorf("%w: %v", errUsage, err)
		}
		return nil
	}
}

// progress returns a logger that writes reader progress to stderr unless
// --quiet or --json is set.
func (rc *runtime) progress() func(string, ...any) {
	if rc.g.quiet || rc.g.asJSON {
		return nil
	}
	return func(format string, args ...any) {
		// Workspace object names are untrusted. Neutralize terminal control sequences at the sink.
		safe := make([]any, len(args))
		for i, arg := range args {
			safe[i] = terminalText(fmt.Sprint(arg))
		}
		_, _ = fmt.Fprintf(rc.stderr, format+"\n", safe...)
	}
}

func commandRequired(cmd *cobra.Command, rc *runtime) error {
	if !rc.g.asJSON {
		_ = cmd.Help()
	}
	return fmt.Errorf("%w: select a command", errUsage)
}

func init() {
	cobra.EnableCommandSorting = false
}
