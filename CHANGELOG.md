# Changelog

All notable changes to this project are recorded in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.3] - 2026-09-11

### Security

- Progress lines, the `auth status` table, and generated HCL comments neutralize control,
  format, and line separator characters that come from workspace object names.
- `inventory.json` and `inventory --json` no longer serialize storage credential secret fields.
- Grant pagination stops with an issue after 1000 pages or a repeated page token.
- Telemetry records `workspace_error` and `render_error` instead of collapsing them to `other`.
- The release job no longer receives the winget token or runs tests with release credentials.
- Release builds use the commit timestamp for the build date and module timestamps.
- The installers require HTTPS and TLS 1.2, and match checksum entries exactly.
- The Windows installer keeps unexpanded `%VAR%` entries in the user PATH.
- `SECURITY.md` lists the reporting channel, response window, and supported versions.

### Changed

- Telemetry is on by default for interactive sessions. CI and agent sessions still send nothing
  unless `DATATF_TELEMETRY=1` is set. `DO_NOT_TRACK`, `DATATF_TELEMETRY=0`, and
  `datatf telemetry disable` still opt out. Invalid consent files still turn telemetry off.
- `inventory` and `export` print a short telemetry notice on stderr before they run and before any
  event is sent, in every output mode, until you save a preference with `datatf telemetry enable`
  or `datatf telemetry disable`.

## [1.0.2] - 2026-09-09

First stable release. The CLI flags, the export file layout, the module contract, and the
generated import addresses are covered by semantic versioning from this release.

Versions 1.0.0 and 1.0.1 were withdrawn before general availability. Start at 1.0.2.

### Added

- Read-only export of supported Azure Databricks workspace and Unity Catalog configuration into
  `terraform.tfvars`, import blocks, and a coverage report.
- `--scaffold` roots for the `536tech/workspace/databricks` pattern module and for the individual
  `536tech` Registry resource modules, both pinned to 1.0.0.
- `--resources`, `--name`, and `--scope` selection with workspace, shared, and system ownership rules.
- `auth status`, `inventory`, `version`, and `telemetry` commands. Telemetry is opt-in.
- Installer scripts for macOS, Linux, and Windows that verify release checksums.
- Release archives with SHA-256 checksums for macOS, Linux, and Windows on AMD64 and ARM64.

[Unreleased]: https://github.com/536tech/datatf/compare/v1.0.3...HEAD
[1.0.3]: https://github.com/536tech/datatf/releases/tag/v1.0.3
[1.0.2]: https://github.com/536tech/datatf/releases/tag/v1.0.2
