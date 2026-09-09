# Security Policy

## Reporting

Report security issues privately through GitHub security advisories or by
contacting the maintainer directly. Do not include credentials in the report.

## Runtime

DataTF reads the selected Databricks workspace and writes files locally. It does not read secret
values or write to Databricks.

Export files can contain names, identifiers, grants, paths, and other workspace metadata. Review
these files before you share them.
