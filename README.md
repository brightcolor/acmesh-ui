<div align="center">

# 🔐 acmesh-ui

**A lightweight, single-binary Web UI for [acme.sh](https://github.com/acmesh-official/acme.sh)**

Manage your TLS certificates from a clean, dark-first admin panel — issue, renew,
install, deploy and monitor — without ever exposing a shell.

[![CI](https://github.com/brightcolor/acmesh-ui/actions/workflows/ci.yml/badge.svg)](https://github.com/brightcolor/acmesh-ui/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/brightcolor/acmesh-ui?sort=semver)](https://github.com/brightcolor/acmesh-ui/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Single Binary](https://img.shields.io/badge/deploy-single%20binary-success)](#-build)
[![No Docker](https://img.shields.io/badge/Docker-not%20required-informational)](#what-it-deliberately-does-not-do)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platforms](https://img.shields.io/badge/linux-amd64%20%7C%20arm64-blue)](#-build)

</div>

---

acmesh-ui does **not** reimplement ACME. It drives an existing `acme.sh`
installation through a small set of **whitelisted, argv-only** commands and gives
you a clean admin panel for the everyday certificate work. The Go backend ships
the entire frontend embedded in the binary — **no Docker, no Node.js at runtime,
no CDN, no external database.**

> [!WARNING]
> Version 1 ships with `auth.mode: none`. The security model is **not** the UI —
> it is the restricted action set **plus** restricting network access. Run it
> behind a VPN, SSH tunnel, or an authenticating reverse proxy.

## ✨ Features

- 📊 **Dashboard** — valid / expiring / expired counts, acme.sh status, recent jobs, next expiries, security warnings.
- 🛡 **Certificates** — scanned from the acme.sh home and parsed with Go `crypto/x509`: SANs, wildcard, expiry, issuer, serial, fingerprint, key type, file paths.
- ➕ **Issue wizard** — HTTP-01 webroot, standalone, DNS-01 via DNS-API, DNS-manual (clearly warned). Live command preview with **masked secrets** before execution.
- ☁ **DNS providers** — env stored **AES-256-GCM encrypted**, masked everywhere, with a curated catalogue (Cloudflare, Hetzner, INWX, Route53, deSEC, netcup, …).
- 📦 **Install & deploy** — `--install-cert` with **allow-listed reload templates** only; `--deploy` with validated hook names.
- 🔄 **Renewals** — single, force, renew-all. **Respects** the existing acme.sh cron/timer — never creates its own.
- ⚙ **Jobs** — background queue (`max_parallel`), **live SSE logs**, cancel, history, automatic failure classification.
- 🌙 **Dark-first UI** — responsive admin panel, light mode toggle, embedded — zero runtime dependencies.

## 🔒 Security model

acmesh-ui assumes a protected admin network. What it enforces in code:

| Guarantee | How |
|-----------|-----|
| **No shell, ever** | `exec.CommandContext(ctx, acmeBinary, args...)` — never `sh -c`, no string concatenation |
| **Allow-listed actions** | only `--list --info --issue --renew --renew-all --install-cert --deploy --set-default-ca --version` |
| **Strict validation** | domains, wildcards, paths (no `..`/null/shell-meta), provider codes, hook names, CA |
| **Reload safety** | `--reloadcmd` only from configured templates (argv arrays) |
| **Secret hygiene** | secrets encrypted at rest, passed via env (never argv), masked in UI/API/logs/preview |
| **Open-bind gate** | non-loopback bind + `auth=none` refuses to start unless explicitly acknowledged |

Full details in [docs/security.md](docs/security.md).

## ⬇️ Install (prebuilt binary)

Binaries for `linux/amd64` and `linux/arm64` are built by GitHub Actions and
attached to every [release](https://github.com/brightcolor/acmesh-ui/releases/latest).
This one-liner picks the right architecture, downloads the latest binary,
verifies its checksum and installs it globally to `/usr/local/bin`:

```sh
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; esac
BASE="https://github.com/brightcolor/acmesh-ui/releases/latest/download"
curl -fsSL "$BASE/acmesh-ui-linux-$ARCH" -o acmesh-ui
curl -fsSL "$BASE/SHA256SUMS" | grep "acmesh-ui-linux-$ARCH" | sed "s| .*| acmesh-ui|" | sha256sum -c -
sudo install -m 0755 acmesh-ui /usr/local/bin/acmesh-ui && rm acmesh-ui
acmesh-ui version
```

> Prefer to build it yourself? See [Build](#-build). Want a specific version?
> Replace `latest/download` with `download/<tag>` (e.g. `download/v1.0.0`).

## 🚀 Quick start

```sh
# 1. acme.sh must already be installed and working
curl https://get.acme.sh | sh -s email=you@example.com

# 2. Install acmesh-ui (see the one-liner above), then create the config
sudo mkdir -p /etc/acmesh-ui /var/lib/acmesh-ui
sudo acmesh-ui init  --config /etc/acmesh-ui/config.yaml
sudo acmesh-ui config check --config /etc/acmesh-ui/config.yaml   # edit acme.binary / acme.home first

# 3. Run (loopback by default)
acmesh-ui serve --config /etc/acmesh-ui/config.yaml
```

Reach it via SSH tunnel:

```sh
ssh -L 8090:127.0.0.1:8090 user@server   # then open http://127.0.0.1:8090
```

A commented sample config lives at [`packaging/config.example.yaml`](packaging/config.example.yaml).
For the systemd service and the acme.sh **permissions model**, see
[docs/install.md](docs/install.md).

## 🖥 CLI

```
acmesh-ui serve        [--config PATH]            Start the web server
acmesh-ui init         [--config PATH] [--force]  Write a sample config
acmesh-ui config check [--config PATH]            Validate the config
acmesh-ui scan         [--config PATH]            Print a certificate summary
acmesh-ui version                                 Version info
```

## 🧱 Architecture

```
cmd/acmesh-ui          CLI entrypoint (serve/init/config/scan/version)
internal/
  acme/                command builder (whitelist), client, parser, scanner
  certs/               x509 parsing + expiry status
  config/              loader, defaults, validation, open-bind gate
  secrets/             AES-256-GCM cipher + central masking
  validate/            domain / wildcard / path / provider / hook checks
  jobs/                worker pool, live SSE logs, cancel, persistence
  dnsproviders/        encrypted provider store + catalogue
  storage/             bbolt (pure Go, no cgo)
  api/                 JSON API handlers
  server/              routing, middleware, lifecycle
  ui/                  go:embed frontend (HTML/CSS/vanilla JS)
packaging/             systemd unit + example config
docs/                  install / security / troubleshooting
```

Storage is [bbolt](https://github.com/etcd-io/bbolt) — pure Go, so the binary
needs **no cgo** and stays a single self-contained artifact.

## 🔧 Build

```sh
make build        # CGO-free single binary (frontend embedded)
make test         # unit + fake-acme.sh integration tests
make release      # linux/amd64 + linux/arm64 into dist/
make checksums    # SHA256SUMS for the release artifacts
```

The frontend ships pre-built under `internal/ui/web/` (plain HTML/CSS/vanilla-JS).
**No Node.js is required to build or run.**

## 💾 Backup / Restore / Upgrade

- **Backup** `/etc/acmesh-ui/` (config + `secret.key`) and `/var/lib/acmesh-ui/` (jobs + DNS DB).
- **Restore** with the **same `secret.key`** — otherwise stored DNS secrets cannot be decrypted.
- **Upgrade** = replace the binary + `systemctl restart acmesh-ui`.

## What it deliberately does NOT do

No Docker/Kubernetes · no external database · no browser terminal · no arbitrary
command runner · no multi-tenant SaaS · no own ACME client · no automatic public
exposure · no forced login · no RBAC (v1). acmesh-ui never creates its own renewal
cron — the existing acme.sh schedule stays authoritative.

## 📄 License

[MIT](LICENSE) © bright-color
