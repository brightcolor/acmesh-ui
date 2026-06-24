# Troubleshooting

Most failures fall into a handful of categories. The Jobs view classifies
common acme.sh errors automatically; the table below maps symptoms to fixes.

## Startup

| Symptom | Cause | Fix |
|---------|-------|-----|
| `refusing to start: server.bind=... auth.mode=none` | Open bind without auth | Set `security.allow_open_without_auth: true`, or bind to `127.0.0.1` |
| `config error: ...` | Invalid YAML or values | `acmesh-ui config check --config ...` |
| `secret key ...` permission denied | Service user cannot write `secret_key_file` | `chown` the `/etc/acmesh-ui` dir to the service user |
| `open db ...` | Data dir not writable | `chown` `/var/lib/acmesh-ui` to the service user |

## acme.sh integration

| Symptom | Cause | Fix |
|---------|-------|-----|
| Header shows "acme.sh fehlt" | Binary not found / not executable | Correct `acme.binary`; `chmod +x`; check the service user can read it |
| Version empty | Binary present but not runnable as the service user | Verify with `sudo -u <user> <acme.binary> --version` |
| No certificates listed | Wrong `acme.home`, or unreadable | Point `acme.home` at the right dir; ensure the service user can read it |
| Cert shows "fehlerhaft" | Cert file unpar. / corrupt | Inspect `*.cer` in the domain dir; re-issue if needed |

## Issuance / renewal

| acme.sh symptom | Meaning | Fix |
|-----------------|---------|-----|
| Rate Limit / rateLimited | CA rate limit hit | Wait; use **Staging** for tests |
| DNS problem | DNS-01 record missing/not propagated | Check DNS provider creds and TTL/propagation |
| Verify error | HTTP-01 challenge unreachable | Check webroot path/permissions and that `/.well-known/acme-challenge/` is served |
| Could not bind / Address already in use | Standalone wants port 80 | Stop the web server briefly or use webroot/DNS instead |
| Webroot not writable | acme.sh can't write the challenge file | Fix ownership/permissions of the webroot |
| Cert already exists | Not due for renewal | Use **Force Renew** if you really must |
| reloadcmd failed | Service reload after install failed | Check `systemctl status <svc>`; verify the template |

## DNS providers

- "DNS provider must be selected" — choose a provider for the DNS-01 challenge.
- Secret shows `********` after saving — expected; secrets are never returned in
  clear. Leaving a secret field empty when editing keeps the previous value.
- acme.sh may also persist some DNS creds in `account.conf` — the UI hints at
  this on the DNS page.

## Jobs

- A job stuck in **queued** — `jobs.max_parallel` slots are busy; it starts when
  one frees up.
- **Cancel** sets the status to `cancelled` and terminates the acme.sh process
  via context.
- Live logs use Server-Sent Events; if your reverse proxy buffers responses,
  disable buffering for `/api/jobs/*/logs` (e.g. nginx `proxy_buffering off;`).

## Logs

```sh
sudo journalctl -u acmesh-ui -f          # service + startup warnings
```

Per-job stdout/stderr (masked) is stored in the bbolt DB and shown in the Job
detail view.
