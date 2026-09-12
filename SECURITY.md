# Security Policy

## Reporting

Report security issues privately through
[GitHub security advisories](https://github.com/536tech/datatf/security/advisories/new).
Do not include credentials in the report. Do not open a public issue for a security problem.

The maintainer acknowledges reports within 7 days and aims to publish a fix within 90 days.

## Supported versions

Only the latest release on the 1.x line receives security fixes.

## Distribution

Release archives ship with a `checksums.txt` file. The installers verify the archive
checksum. Archives are not yet signed. Verify the checksum from the release page before you
use a manually downloaded archive.

## Runtime

DataTF reads the selected Databricks workspace and writes files locally. It does not read secret
values or write to Databricks.

Export files can contain names, identifiers, grants, paths, and other workspace metadata. Review
these files before you share them.
