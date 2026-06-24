# Security model

acmesh-ui assumes it runs in a **protected admin network**. Version 1 has no
internal login. Security therefore rests on two pillars:

1. **Restrict who can reach the app** (VPN / SSH tunnel / reverse proxy / loopback).
2. **Restrict what the app can do**, regardless of who reaches it.

This document is about pillar 2 — the part acmesh-ui enforces in code.

## No shell, ever

Commands are built as **argv slices** and executed with
`exec.CommandContext(ctx, acmeBinary, args...)`. There is no `sh -c`, no string
concatenation into a shell, and the only executable invoked is the configured
`acme.sh` binary.

```go
// internal/acme/command_builder.go
exec.CommandContext(ctx, acmeBinary, "--issue", "-d", domain, "-w", webroot)
```

## Allow-listed actions only

The command builder can only produce these acme.sh actions:

`--list`, `--info`, `--issue`, `--renew`, `--renew-all`, `--install-cert`,
`--deploy`, `--set-default-ca`, `--version`.

There is no endpoint that accepts an arbitrary acme.sh flag or argument.

## Strict input validation (`internal/validate`)

- **Domains:** RFC-ish label rules, no spaces, no shell metacharacters, not a
  URL, must have ≥2 labels.
- **Wildcards:** exactly `*.example.com` (single leading `*.`).
- **Paths:** absolute, no null bytes, no `..` traversal, no `$()`/backticks/`;`
  /`|`/`&&`; optional allow-listed base directories.
- **Provider codes:** `dns_[a-z0-9_]+`.
- **Hook names / env names:** restricted character sets.
- **CA:** known shortname or an `https://` directory URL only.

## Reload commands

`--install-cert --reloadcmd` is the riskiest surface (acme.sh runs it via a
shell). acmesh-ui therefore:

- Offers only **allow-listed templates** from `config.yaml`
  (`reload_commands`, argv arrays).
- Rejects any reload command that was not matched against that list
  (`reloadAllowed=false` ⇒ error), unless `security.allow_free_reloadcmd` is
  explicitly enabled.
- Never lets a secret flow into a reload command.

## Secret handling

- DNS provider env values flagged as secret are encrypted with **AES-256-GCM**
  (key in `security.secret_key_file`, auto-created `0600`).
- Secrets are passed to acme.sh via the **process environment**, never as argv,
  and never written to the persisted job record.
- A central masker (`internal/secrets`) registers every known secret value and
  redacts it — plus heuristic redaction of `Authorization`/`Bearer` headers and
  secret-looking `NAME=value` lines — across UI, API responses, job logs, the
  command preview, and error messages.
- API responses return secret env values as `********` and never in clear text.

## Open-bind gate

If `server.bind` is a non-loopback address **and** `auth.mode: none`, acmesh-ui
**refuses to start** unless `security.allow_open_without_auth: true` is set.
Loopback binds always start. The UI shows a persistent (non-intrusive) warning
whenever auth is disabled, and a stronger one when bound openly.

## HTTP hardening

`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Referrer-Policy: no-referrer`, request-body size limits, panic recovery, and a
read-header timeout are applied to every request.

## What you still must do

- Keep the app off the public internet (or put real auth in front).
- Protect `secret.key` and the data directory with filesystem permissions.
- Run as a least-privileged user (see `docs/install.md`).
