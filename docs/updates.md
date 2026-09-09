# Upgrade DataTF

DataTF shows an update notice when a newer stable release is available.
It does not install updates or change your Terraform files.

1. Open the release link in the notice.
2. Download the archive for your system and processor.
3. Compare its SHA-256 checksum with the value in `checksums.txt` from the same release.
4. Extract the archive.
5. Replace your installed `datatf` binary, or `datatf.exe` on Windows.
6. Run `datatf version` to check the installed version.

Use `command -v datatf` on macOS or Linux to find the installed binary.
Use `where.exe datatf` in PowerShell on Windows.
If your company manages the installation, ask its administrator to approve the update.

## Check behavior

DataTF checks after a successful interactive command, help output, or a launch without a command.
It writes the notice to stderr. It does not change stdout, the exit code, or export files.
Only stable builds check for updates. Development builds and prereleases do not check.

Checks require terminal output on both stdout and stderr.
JSON, plain, quiet, completion, and telemetry commands do not check.
Detected CI and agent sessions do not check.
DataTF uses the same CI and agent indicators listed in the [telemetry notice](telemetry.md).

Results and failed attempts stay in a local cache for 24 hours.
The cache contains only a check time and a release version.
DataTF stores it in `datatf/update.json` under the operating system's user cache directory.
If that directory is unavailable, DataTF skips the check.

Each check has a 500-millisecond network limit. Network failures produce no notice.
There are no retries or redirects. Normal HTTPS certificate checks and proxy settings apply.

## Network access and controls

Update checks are separate from optional telemetry.
They request public release metadata from `api.github.com` without authentication.
They send no installed version, workspace data, profile, command arguments, or credentials.
GitHub receives the request's network metadata, including an IP address.

To disable checks in the current shell:

```sh
export DATATF_NO_UPDATE_NOTIFIER=1
```

In PowerShell:

```powershell
$env:DATATF_NO_UPDATE_NOTIFIER = '1'
```

Any nonempty `DATATF_NO_UPDATE_NOTIFIER` value disables the check and the notice.
`DO_NOT_TRACK=1` or `DO_NOT_TRACK=true` also disables both update checks and telemetry.
`DATATF_TELEMETRY=0` disables usage events only. It does not control update checks.

You can always check [GitHub Releases](https://github.com/536tech/datatf/releases/latest) yourself.
