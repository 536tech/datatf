# Changelog

All notable changes to this project are recorded in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/536tech/datatf/compare/v1.0.2...HEAD
[1.0.2]: https://github.com/536tech/datatf/releases/tag/v1.0.2
