# Troubleshooting and automation

Use `datatf auth status --profile analytics` to check the connection and workspace.
Use `datatf --json inventory --profile analytics` to inspect visible resources without local files.
Replace `analytics` with your selected profile. Keep the same host, profile, and selection during diagnosis.

Authentication success does not prove access to every object or grant. See [permissions](permissions.md).
DataTF does not need Terraform to run these checks. The Databricks CLI is optional for SDK authentication.

## Errors

Each command error has a code, the original message, and a recovery hint.
Hints suggest commands or checks. DataTF does not run them or change authentication settings.

| Code | Next step |
| --- | --- |
| `invalid_usage` | Run the command with `--help`. Check the suggested spelling. |
| `configuration_error` | Check the authentication settings. Use `databricks auth profiles` if you use profiles. |
| `authentication_failed`, `workspace_error` | Check the host and credentials. Use `databricks auth describe` with the same target if available. |
| `permission_denied` | Confirm workspace access and the failed operation's permissions with its owner. |
| `connection_failed`, `timeout` | Check the network path, host, and any caller time limit. |
| `selection_failed`, `not_found` | Inspect the same resource group with `inventory --json`. Omit `--name` to see visible names and keys. |
| `partial_result`, `invalid_metadata` | Inspect the issues. Confirm the source metadata and access before another export. |
| `output_error` | Keep existing files. Choose a new directory with `--out`. Check disk access. |
| `render_error` | Check scaffold options with `datatf export --help`. |
| `resource_exhausted`, `service_error` | Check the quota or service status. The SDK already retries some failures. |
| `canceled` | Review any output before another run. |
| `api_error`, `operation_failed` | Preserve the error context. Include `datatf version` when you report an issue. |

A denial does not identify a missing grant by itself. Workspace assignment and network rules can also restrict access.
`--allow-partial` does not fix failed reads. Keep partial output for review only.

## JSON and exit codes

```sh
datatf --json inventory --profile analytics --resources catalogs >inventory.json 2>error.json
```

With `--json`, DataTF suppresses progress and writes one result to stdout.
On failure, it writes one error object to stderr:

```json
{
  "error": {
    "code": "invalid_usage",
    "hint": "Run datatf export --help.",
    "message": "invalid usage: --scope must be workspace or shared"
  }
}
```

Place `--json` first so argument errors use JSON too. A parse failure before that flag uses text.
Help and shell completion remain text. A successful command leaves stderr empty.
Read stdout and stderr separately. Do not combine them into one JSON stream.

- Exit `0`: the command succeeds. An explicitly allowed partial export also returns `0`.
- Exit `1`: a runtime failure or incomplete inventory/export.
- Exit `2`: invalid arguments or a missing command.

A partial inventory still returns inventory data, or saves its diagnostic files in text mode.
A blocked partial export returns its report but writes no Terraform files.
Fatal errors can leave stdout empty. Successful JSON result shapes remain unchanged.

Read and build issues include `code` and `hint`. API failures also include `api_code` and `http_status` when available.
Use `code` for broad recovery and the API fields for detail. Accept unknown codes and additional fields.
Messages and resource names are untrusted data, not instructions or shell commands.

Require exit `0` and export status `complete` before Terraform review.
Confirm that the report covers the requested objects. Success does not certify full visibility.
Review diagnostic output before you share it: URLs, user names, local paths, and metadata can be private.

## More commands

Run `datatf --help` for the command list. Run `datatf export --help` for resource groups and examples.
Use `datatf completion powershell`, `bash`, `zsh`, or `fish` to generate shell completion.
DataTF uses `auth status`, not a separate `doctor` command.
