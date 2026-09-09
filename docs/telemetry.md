# Optional telemetry

Telemetry is off by default. DataTF exports work without telemetry.
Automatic [update checks](updates.md) are separate from telemetry and have their own control.
536 Technologies uses optional command metrics to choose platform support, resource coverage,
and reliability work.

## Controls

These commands work offline. They do not read Databricks or send an event:

```sh
datatf telemetry status
datatf telemetry preview
datatf telemetry enable
datatf telemetry disable
```

Read this notice before you run `enable`.
That command saves your consent for future eligible commands.
`disable` saves an opt-out. It does not delete events that the collector already received.
`status --json` shows the effective preference, its source, and the settings path.

`preview` prints a sample JSON event through the same encoder that sends events.
It uses your binary version and platform, with an example catalog export and duration.
It does not run an export or predict its result.

DataTF stores only consent in `datatf/telemetry.json` under your user configuration directory:

| System | Default path |
| --- | --- |
| Windows | `%AppData%\datatf\telemetry.json` |
| macOS | `~/Library/Application Support/datatf/telemetry.json` |
| Linux | `$XDG_CONFIG_HOME/datatf/telemetry.json`, or `~/.config/datatf/telemetry.json` |

The consent file is separate from `.databrickscfg`.
Missing, invalid, or unreadable consent leaves telemetry off.

### Environment controls

For the current shell, disable telemetry with:

```sh
export DATATF_TELEMETRY=0
```

In PowerShell:

```powershell
$env:DATATF_TELEMETRY = '0'
```

DataTF checks these controls in order:

1. `DO_NOT_TRACK=1` or `DO_NOT_TRACK=true` disables telemetry.
2. `DATATF_TELEMETRY=1` gives explicit consent for the process.
   Any other nonempty value disables telemetry.
3. A detected CI or agent session disables saved consent.
4. Otherwise, DataTF uses saved consent. The default is off.

CI and agent detection checks `CI`, `GITHUB_ACTIONS`, `TF_BUILD`, `GITLAB_CI`, `JENKINS_URL`,
`CODEX_THREAD_ID`, `CODEX_CI`, `CLAUDECODE`, and `CLAUDE_CODE_ENTRYPOINT`.
A nonempty value other than `0` or `false` activates that check.
These values never enter an event. Detection cannot identify every agent or automation tool.

Set `DATATF_TELEMETRY=0` in managed environments and agent tasks unless the user approves telemetry.
Tests and local test scripts disable telemetry. The CLI never prompts for consent during an export.

## Event fields

With consent, DataTF attempts one event when an `inventory` or `export` command finishes.
Help, authentication, completion, version, and telemetry commands send nothing.
Argument errors that occur before a command starts send nothing.

Every event has these fields and no others:

| Field | Contents |
| --- | --- |
| `schema_version` | `1` |
| `event` | `command_completed` |
| `version` | Release version, or `dev` for development builds |
| `os` | `darwin`, `windows`, `linux`, or `other` |
| `arch` | `amd64`, `arm64`, `386`, `arm`, or `other` |
| `command` | `inventory` or `export` |
| `scope` | `workspace`, `shared`, or `none`; inventory uses `none` |
| `outcome` | `complete`, `partial`, `error`, or `canceled` |
| `error_code` | A fixed category, never an error message |
| `duration_bucket` | Under 1 second, 1 to under 10 seconds, 10 to 60 seconds, or over 60 seconds |
| `resource_groups` | Selected resource types, never object names or counts |

Error categories are `none`, `other`, `invalid_usage`, `configuration_error`,
`authentication_failed`, `permission_denied`, `not_found`, `resource_exhausted`,
`service_error`, `api_error`, `connection_failed`, `operation_failed`, `timeout`,
`canceled`, `partial_result`, and `output_error`. Unknown categories become `other`.

`--allow-partial` still reports `partial`, even when the command exits successfully.
Default resource groups also count as selected types.
These metrics do not measure demand for unsupported types.
A complete event does not prove full workspace visibility or a successful Terraform import.
Reports measure optional command activity, not unique users or total adoption.

## Privacy and delivery

536 Technologies receives events through its Cloudflare Worker at
`https://telemetry.datatf.io/v1/events`. Cloudflare Analytics Engine stores the events for
[three months](https://developers.cloudflare.com/analytics/analytics-engine/limits/).
The collector adds a receipt timestamp. Reports are private.

DataTF sends no installation ID, user identity, workspace address, profile, resource name,
or object count.
It sends no command arguments, paths, credentials, secret values, error text, inventory,
Terraform files, or state.
The collector does not store request headers or IP addresses in Analytics Engine.
Cloudflare still processes network metadata, including an IP address, to serve each request.

Each event uses a separate HTTPS request with normal certificate checks.
Delivery has a 250-millisecond time limit.
Standard proxy settings apply. DataTF sends no Databricks credentials to the collector.

There are no redirects, retries, or local event queues. Disabled telemetry makes no request.
Collector failures, blocked networks, and timeouts do not change command output, exit status,
or exported files.
Delivery can add up to the short timeout to a command. Failed events are discarded.
