# Installation & operations

## 1. Prerequisites

- Linux server with **acme.sh** already installed and able to issue certs.
- Know where acme.sh lives (`acme.binary` and `acme.home`). Common locations:
  - `/root/.acme.sh/acme.sh` (root install)
  - `/home/acme/.acme.sh/acme.sh` (dedicated user — recommended)

## 2. The permissions problem (read this)

acme.sh needs to read/write its home, touch webroots, and (for install) write
cert files and run reload commands. acmesh-ui runs acme.sh as **its own
process user**, so that user must have the right access. Pick one model:

### Model A — dedicated `acme` user (recommended)

```sh
sudo useradd --system --create-home --home-dir /home/acme --shell /bin/bash acme
sudo -u acme sh -c 'curl https://get.acme.sh | sh -s email=you@example.com'
# acme.sh home is now /home/acme/.acme.sh, owned by acme.
```

Run acmesh-ui **as that same `acme` user** so it can drive acme.sh directly.
In the unit file set `User=acme` / `Group=acme` and point the config at
`/home/acme/.acme.sh/acme.sh`.

### Model B — separate `acmesh-ui` user with limited sudo

Keep acme.sh under `acme`, run the UI as `acmesh-ui`, and grant a **narrow**
sudo rule. Because acmesh-ui only ever calls the binary with a fixed argv, you
can scope sudo to the binary:

```sudoers
# /etc/sudoers.d/acmesh-ui
acmesh-ui ALL=(acme) NOPASSWD: /home/acme/.acme.sh/acme.sh
```

Then set `acme.binary` to a small wrapper that prepends `sudo -u acme`.
(A future version may support a configurable command prefix natively.)

### Model C — root (simplest, least clean)

Run the service as `root` with acme.sh under `/root/.acme.sh`. Easy, but the
web process then runs as root — only acceptable on a tightly access-controlled
host. Document it, don't default to it.

> Recommendation: **Model A**. One `acme` user owns acme.sh and runs the UI.

## 3. Directories

| Path | Purpose | Owner |
|------|---------|-------|
| `/etc/acmesh-ui/config.yaml` | configuration | service user (read) |
| `/etc/acmesh-ui/secret.key` | AES key for DNS secrets (auto-created 0600) | service user |
| `/var/lib/acmesh-ui/` | bbolt DB (jobs, DNS providers) | service user (rw) |

```sh
sudo mkdir -p /etc/acmesh-ui /var/lib/acmesh-ui
sudo chown -R acme:acme /var/lib/acmesh-ui /etc/acmesh-ui
```

## 4. Configure

```sh
sudo -u acme acmesh-ui init --config /etc/acmesh-ui/config.yaml
sudo -u acme acmesh-ui config check --config /etc/acmesh-ui/config.yaml
```

Key fields: `acme.binary`, `acme.home`, `server.bind`, `auth.mode`,
`reload_commands`. See `packaging/config.example.yaml`.

## 5. systemd

Edit `packaging/systemd/acmesh-ui.service` to match your user model
(`User=`/`Group=`), then:

```sh
sudo cp packaging/systemd/acmesh-ui.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now acmesh-ui
sudo systemctl status acmesh-ui
sudo journalctl -u acmesh-ui -f
```

The startup log prints an explicit warning when `auth.mode=none`.

## 6. Access patterns

- **SSH tunnel:** `ssh -L 8090:127.0.0.1:8090 user@server` → `http://127.0.0.1:8090`
- **VPN:** set `server.bind` to the VPN IP; set
  `security.allow_open_without_auth: true` to acknowledge open-without-auth.
- **Reverse proxy:** keep `bind: 127.0.0.1`, proxy from nginx/Caddy/Traefik and
  add Basic Auth or Cloudflare Access in front.

## 7. Renewals

acme.sh already installs its own cron job / systemd timer for the user it runs
as. **acmesh-ui does not create or manage any renewal schedule.** The
Systemstatus page surfaces any renewal mechanisms it can detect so you can spot
overlaps. Use the UI's renew buttons for ad-hoc/forced renewals only.

## 8. Backup / Restore / Upgrade

- Backup `/etc/acmesh-ui/` (incl. `secret.key`) and `/var/lib/acmesh-ui/`.
- Restore with the **same secret.key** or stored DNS secrets become
  undecryptable (you'd have to re-enter them).
- Upgrade: replace `/usr/local/bin/acmesh-ui`, then
  `sudo systemctl restart acmesh-ui`.
