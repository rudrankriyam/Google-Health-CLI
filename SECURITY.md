# Security Policy

`ghealth` handles health data and OAuth tokens, so security reports are taken seriously.

## Supported Versions

Only the latest released version is supported.

## Reporting

Please report security issues privately by email to the repository owner. Do not open a public issue with token material, private health data, OAuth credentials, or API responses containing personal data.

## Local Secrets

`ghealth` stores OAuth tokens in the user config directory with file mode `0600`. Do not commit local config or token files.
