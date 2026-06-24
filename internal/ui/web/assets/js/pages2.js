// Additional pages, merged into the Pages object: new-cert wizard, DNS, jobs, settings, system.
Object.assign(Pages, (() => {
  const el = () => document.getElementById('content');
  const set = (html) => { el().innerHTML = html; };

  // ---------- New certificate wizard ----------
  let wiz = { step: 1, dnsProviders: [], settings: null };
  async function newCert() {
    set('<div class="loading">Lade…</div>');
    try {
      const d = await API.dnsProviders();
      wiz.dnsProviders = d.providers || [];
      wiz.settings = await API.settings();
    } catch (e) { return UI.apiError(e); }
    wiz.step = 1;
    renderWizard();
  }

  function renderWizard() {
    const def = wiz.settings || { acme: {} };
    const html = `
      <div class="steps">
        ${['Domains', 'Challenge', 'Optionen', 'Vorschau'].map((s, i) =>
          `<span class="step ${wiz.step === i + 1 ? 'active' : ''}">${i + 1}. ${s}</span>`).join('')}
      </div>
      <div class="card">
        <form id="wizform">
          <div class="form-row"><label>Hauptdomain</label><input type="text" id="w-main" placeholder="example.com" required></div>
          <div class="form-row"><label>Zusätzliche Domains (eine pro Zeile)</label><textarea id="w-sans" placeholder="www.example.com&#10;api.example.com"></textarea></div>
          <div class="form-row checkbox"><input type="checkbox" id="w-wild"><label style="margin:0">Wildcard <span class="tag">*.example.com</span> hinzufügen (benötigt DNS-01)</label></div>

          <div class="form-row"><label>Validierungsmethode</label>
            <select id="w-challenge">
              <option value="webroot">HTTP-01 Webroot</option>
              <option value="standalone">Standalone (Port 80)</option>
              <option value="dns">DNS-01 über DNS-API</option>
              <option value="dns-manual">DNS-Manual (nicht für Auto-Renew!)</option>
            </select>
          </div>
          <div class="form-row" id="w-webroot-row"><label>Webroot-Pfad</label><input type="text" id="w-webroot" value="${UI.esc(def.acme.default_webroot || '')}" placeholder="/var/www/example/web"></div>
          <div class="form-row" id="w-dns-row" style="display:none"><label>DNS-Provider</label>
            <select id="w-dns">${wiz.dnsProviders.map(p => `<option value="${UI.esc(p.id)}">${UI.esc(p.name)} (${UI.esc(p.code)})</option>`).join('')}</select>
            <div class="help">${wiz.dnsProviders.length ? '' : 'Noch kein DNS-Provider angelegt — unter „DNS-Provider“ konfigurieren.'}</div>
          </div>
          <div id="w-challenge-warn"></div>

          <div class="grid grid-2">
            <div class="form-row"><label>Key-Typ</label><select id="w-key">
              ${['ec-256', 'ec-384', '2048', '4096'].map(k => `<option ${k === (def.acme.default_key_type || 'ec-256') ? 'selected' : ''}>${k}</option>`).join('')}
            </select></div>
            <div class="form-row"><label>CA</label><select id="w-ca">
              <option value="">Standard (acme.sh)</option>
              <option value="letsencrypt">Let's Encrypt</option>
              <option value="letsencrypt_test">Let's Encrypt (Staging)</option>
              <option value="zerossl">ZeroSSL</option>
              <option value="buypass">Buypass</option>
              <option value="google">Google</option>
            </select></div>
          </div>
          <div class="btn-row">
            <div class="checkbox"><input type="checkbox" id="w-staging"><label style="margin:0">Staging/Test</label></div>
            <div class="checkbox"><input type="checkbox" id="w-force"><label style="margin:0">Force</label></div>
          </div>

          <div id="w-preview" style="margin-top:16px"></div>
          <div class="btn-row" style="margin-top:18px">
            <button type="button" class="btn" id="w-preview-btn">Vorschau erzeugen</button>
            <button type="button" class="btn btn-primary" id="w-submit-btn" disabled>Ausstellung starten</button>
          </div>
        </form>
      </div>`;
    set(html);

    const chSel = document.getElementById('w-challenge');
    const updateChallenge = () => {
      const v = chSel.value;
      document.getElementById('w-webroot-row').style.display = v === 'webroot' ? '' : 'none';
      document.getElementById('w-dns-row').style.display = (v === 'dns') ? '' : 'none';
      const warn = document.getElementById('w-challenge-warn');
      if (v === 'dns-manual') warn.innerHTML = '<div class="alert alert-danger alert-inline"><strong>DNS-Manual</strong> erfordert manuelles Setzen der TXT-Records und ist <u>nicht für automatische Renewals</u> geeignet.</div>';
      else if (v === 'standalone') warn.innerHTML = '<div class="alert alert-warn alert-inline">Standalone bindet <strong>Port 80</strong> — dieser muss frei sein (z. B. Webserver kurz stoppen).</div>';
      else warn.innerHTML = '';
      document.getElementById('w-submit-btn').disabled = true;
    };
    chSel.onchange = updateChallenge;
    document.getElementById('w-wild').onchange = (e) => { if (e.target.checked) { chSel.value = 'dns'; updateChallenge(); } };
    ['w-main', 'w-sans', 'w-webroot', 'w-dns', 'w-key', 'w-ca', 'w-staging', 'w-force'].forEach(id => {
      const node = document.getElementById(id); if (node) node.addEventListener('change', () => document.getElementById('w-submit-btn').disabled = true);
    });
    document.getElementById('w-preview-btn').onclick = () => doPreview();
    document.getElementById('w-submit-btn').onclick = () => doIssue();
    updateChallenge();
  }

  function collectIssue() {
    const main = document.getElementById('w-main').value.trim();
    const sans = document.getElementById('w-sans').value.split('\n').map(s => s.trim()).filter(Boolean);
    const domains = [main, ...sans];
    if (document.getElementById('w-wild').checked) domains.push('*.' + main);
    return {
      domains,
      challenge: document.getElementById('w-challenge').value,
      webroot: document.getElementById('w-webroot').value.trim(),
      dns_provider_id: document.getElementById('w-dns') ? document.getElementById('w-dns').value : '',
      key_type: document.getElementById('w-key').value,
      ca: document.getElementById('w-ca').value,
      staging: document.getElementById('w-staging').checked,
      force: document.getElementById('w-force').checked,
    };
  }
  async function doPreview() {
    const body = collectIssue(); body.preview = true;
    try {
      const r = await API.issue(body);
      document.getElementById('w-preview').innerHTML =
        `<div class="card-title">Befehlsvorschau (Secrets maskiert)</div>${UI.codeblock(r.preview, true)}`;
      document.getElementById('w-submit-btn').disabled = false;
    } catch (e) { UI.apiError(e); }
  }
  async function doIssue() {
    const body = collectIssue();
    const previewEl = document.getElementById('w-preview');
    UI.modal({
      title: 'Zertifikat ausstellen',
      confirmLabel: 'Ausstellung starten',
      bodyHtml: `<p>Domains: ${body.domains.map(d => `<span class="tag">${UI.esc(d)}</span>`).join(' ')}</p>
        <p>Challenge: <span class="tag">${UI.esc(body.challenge)}</span> · Key: <span class="tag">${UI.esc(body.key_type)}</span></p>
        ${previewEl.innerHTML}`,
      onConfirm: async () => {
        const r = await API.issue(body);
        UI.toast('Ausstellung gestartet', 'ok');
        location.hash = '#/jobs/' + r.job_id;
      },
    });
  }

  // ---------- DNS providers ----------
  async function dns() {
    set('<div class="loading">Lade…</div>');
    let d;
    try { d = await API.dnsProviders(); } catch (e) { return UI.apiError(e); }
    const known = d.known || [];
    let html = `<div class="page-head"><h2>DNS-Provider</h2>
      <button class="btn btn-primary" onclick="Pages.dnsEdit(null, ${JSON.stringify(JSON.stringify(known)).replace(/"/g, '&quot;')})">＋ Provider anlegen</button></div>
      <div class="alert alert-info alert-inline">Hinweis: acme.sh speichert manche DNS-Zugangsdaten zusätzlich selbst in <span class="tag">account.conf</span>.</div>`;
    if (!d.providers || !d.providers.length) {
      html += '<div class="card"><div class="empty"><div class="big">☁</div>Noch kein DNS-Provider konfiguriert.</div></div>';
    } else {
      html += '<div class="grid grid-2">' + d.providers.map(p => `
        <div class="card"><div class="page-head"><h3 class="mono">${UI.esc(p.name)}</h3><span class="tag">${UI.esc(p.code)}</span></div>
          ${p.description ? `<p class="muted">${UI.esc(p.description)}</p>` : ''}
          <dl class="kv">${(p.env || []).map(e => `<dt class="mono">${UI.esc(e.name)}</dt><dd class="mono">${e.secret ? '••••••••' : UI.esc(e.value)}</dd>`).join('')}</dl>
          <div class="btn-row" style="margin-top:12px">
            <button class="btn btn-sm" onclick='Pages.dnsEdit(${JSON.stringify(p)}, ${JSON.stringify(JSON.stringify(known))})'>Bearbeiten</button>
            <button class="btn btn-sm btn-danger" onclick="Pages.dnsDelete('${UI.esc(p.id)}','${UI.esc(p.name)}')">Löschen</button>
          </div></div>`).join('') + '</div>';
    }
    set(html);
  }

  function dnsEdit(provider, knownJson) {
    const known = typeof knownJson === 'string' ? JSON.parse(knownJson) : knownJson;
    const isEdit = !!provider;
    const envRows = () => {
      const rows = (provider && provider.env) ? provider.env : [];
      return rows.map(e => envRowHtml(e.name, e.secret ? '' : e.value, e.secret)).join('');
    };
    UI.modal({
      title: isEdit ? 'DNS-Provider bearbeiten' : 'DNS-Provider anlegen',
      confirmLabel: isEdit ? 'Speichern' : 'Anlegen',
      bodyHtml: `
        <div class="form-row"><label>Name</label><input type="text" id="p-name" value="${UI.esc(provider ? provider.name : '')}" placeholder="Cloudflare Main"></div>
        <div class="form-row"><label>Provider-Code</label>
          <input type="text" id="p-code" list="known-codes" value="${UI.esc(provider ? provider.code : '')}" placeholder="dns_cf">
          <datalist id="known-codes">${known.map(k => `<option value="${UI.esc(k.code)}">${UI.esc(k.label)}</option>`).join('')}</datalist>
          <div class="help">z. B. dns_cf, dns_hetzner, dns_inwx</div></div>
        <div class="form-row"><label>Beschreibung</label><input type="text" id="p-desc" value="${UI.esc(provider ? provider.description : '')}"></div>
        <div class="form-row"><label>ENV-Variablen</label><div id="p-env">${envRows()}</div>
          <button type="button" class="btn btn-sm" id="p-add-env">+ Variable</button>
          <div class="help">Secret-Felder werden verschlüsselt gespeichert und nie im Klartext zurückgegeben. Leeres Secret beim Bearbeiten behält den alten Wert.</div></div>`,
      onRender: (body) => {
        body.querySelector('#p-add-env').onclick = () => {
          const div = document.createElement('div');
          div.innerHTML = envRowHtml('', '', true);
          body.querySelector('#p-env').appendChild(div.firstElementChild);
        };
        const codeInput = body.querySelector('#p-code');
        codeInput.addEventListener('change', () => {
          const k = known.find(x => x.code === codeInput.value);
          if (k && !body.querySelector('#p-env').children.length) {
            const cont = body.querySelector('#p-env');
            (k.secret_vars || []).forEach(v => cont.insertAdjacentHTML('beforeend', envRowHtml(v, '', true)));
            (k.plain_vars || []).forEach(v => cont.insertAdjacentHTML('beforeend', envRowHtml(v, '', false)));
          }
        });
      },
      onConfirm: async () => {
        const env = {}; const secret_names = [];
        document.querySelectorAll('#p-env .env-row').forEach(row => {
          const name = row.querySelector('.env-name').value.trim();
          const val = row.querySelector('.env-val').value;
          const sec = row.querySelector('.env-secret').checked;
          if (!name) return;
          if (sec) secret_names.push(name);
          if (!sec || val !== '') env[name] = val;
        });
        const payload = {
          name: document.getElementById('p-name').value.trim(),
          code: document.getElementById('p-code').value.trim(),
          description: document.getElementById('p-desc').value.trim(),
          env, secret_names,
        };
        if (isEdit) await API.updateDNS(provider.id, payload); else await API.createDNS(payload);
        UI.toast('Gespeichert', 'ok');
        Pages.dns();
      },
    });
  }
  function envRowHtml(name, val, secret) {
    return `<div class="env-row inline" style="margin-bottom:6px">
      <input type="text" class="env-name" placeholder="CF_Token" value="${UI.esc(name)}" style="flex:1">
      <input type="${secret ? 'password' : 'text'}" class="env-val" placeholder="${secret ? '(geheim)' : 'Wert'}" value="${UI.esc(val)}" style="flex:1.4">
      <label class="checkbox" style="white-space:nowrap"><input type="checkbox" class="env-secret" ${secret ? 'checked' : ''}>secret</label>
    </div>`;
  }
  function dnsDelete(id, name) {
    UI.modal({
      title: 'DNS-Provider löschen', danger: true, confirmLabel: 'DNS-Provider löschen',
      bodyHtml: `<p>Provider <span class="tag">${UI.esc(name)}</span> wirklich löschen? Gespeicherte Secrets werden entfernt.</p>`,
      onConfirm: async () => { await API.deleteDNS(id); UI.toast('Gelöscht', 'ok'); Pages.dns(); },
    });
  }

  // ---------- Jobs ----------
  let jobFilter = '';
  async function jobs() {
    set('<div class="loading">Lade…</div>');
    let d;
    try { d = await API.jobs(jobFilter ? '?status=' + jobFilter : ''); } catch (e) { return UI.apiError(e); }
    const states = ['', 'running', 'success', 'failed', 'cancelled'];
    const labels = { '': 'Alle', running: 'Läuft', success: 'Erfolg', failed: 'Fehler', cancelled: 'Abgebrochen' };
    let html = `<div class="page-head"><h2>Jobs</h2>
      <div class="filterchips">${states.map(s => `<span class="chip ${jobFilter === s ? 'active' : ''}" data-s="${s}">${labels[s]}</span>`).join('')}</div></div>`;
    if (!d.jobs || !d.jobs.length) {
      html += '<div class="card"><div class="empty"><div class="big">⚙</div>Keine Jobs.</div></div>';
    } else {
      html += `<div class="card"><div class="table-wrap"><table class="tbl">
        <thead><tr><th>Status</th><th>Typ</th><th>Domain</th><th>Start</th><th>Dauer</th><th>Exit</th><th></th></tr></thead><tbody>` +
        d.jobs.map(j => `<tr>
          <td>${UI.jobBadge(j.status)}</td><td>${UI.esc(j.type)}</td><td class="mono">${UI.esc(j.domain || '–')}</td>
          <td class="muted nowrap">${UI.fmtDate(j.started_at)}</td>
          <td class="muted">${UI.fmtDuration(jobDur(j))}</td>
          <td class="mono">${j.status === 'queued' || j.status === 'running' ? '–' : j.exit_code}</td>
          <td><a class="btn btn-sm" href="#/jobs/${UI.esc(j.id)}">Details</a></td></tr>`).join('') +
        '</tbody></table></div></div>';
    }
    set(html);
    document.querySelectorAll('.chip[data-s]').forEach(ch => ch.onclick = () => { jobFilter = ch.dataset.s; jobs(); });
  }
  function jobDur(j) {
    if (!j.started_at || j.started_at.startsWith('0001')) return 0;
    const end = (j.ended_at && !j.ended_at.startsWith('0001')) ? new Date(j.ended_at) : new Date();
    return (end - new Date(j.started_at)) * 1e6;
  }

  // ---------- Job detail + live logs ----------
  let activeSSE = null;
  async function jobDetail(id) {
    if (activeSSE) { activeSSE.close(); activeSSE = null; }
    set('<div class="loading">Lade…</div>');
    let d;
    try { d = await API.job(id); } catch (e) { return UI.apiError(e); }
    const j = d.job;
    const running = j.running || j.status === 'running';
    let html = `<div class="page-head"><h2>Job ${UI.jobBadge(j.status)}</h2>
      <div class="btn-row">
        ${running ? `<button class="btn btn-danger" id="job-cancel">Abbrechen</button>` : ''}
        <a class="btn" href="#/jobs">← Zurück</a>
      </div></div>
      ${j.summary ? `<div class="alert ${j.status === 'failed' ? 'alert-danger' : 'alert-info'} alert-inline">${UI.esc(j.summary)}</div>` : ''}
      <div class="card"><dl class="kv">
        <dt>Typ</dt><dd>${UI.esc(j.type)}</dd>
        <dt>Domain</dt><dd class="mono">${UI.esc(j.domain || '–')}</dd>
        <dt>Start</dt><dd>${UI.fmtDate(j.started_at)}</dd>
        <dt>Ende</dt><dd>${UI.fmtDate(j.ended_at)}</dd>
        <dt>Exit-Code</dt><dd class="mono">${running ? '–' : j.exit_code}</dd>
        <dt>Befehl</dt><dd>${UI.codeblock(j.preview_cmd || '', true)}</dd>
      </dl></div>
      <div class="card"><div class="page-head"><div class="card-title">Log (Secrets maskiert)</div>
        <div class="inline"><label class="checkbox"><input type="checkbox" id="autoscroll" checked>Auto-Scroll</label>
        ${UI.copyBtn('')}</div></div>
        <div class="logview" id="logview">${(j.log || []).map(l => `<span class="ln">${UI.esc(l)}</span>`).join('')}</div>
      </div>`;
    set(html);

    const logview = document.getElementById('logview');
    const autoscroll = () => { if (document.getElementById('autoscroll').checked) logview.scrollTop = logview.scrollHeight; };
    autoscroll();
    const cancelBtn = document.getElementById('job-cancel');
    if (cancelBtn) cancelBtn.onclick = () => Pages.cancelJob(id);

    if (running) {
      activeSSE = new EventSource('/api/jobs/' + encodeURIComponent(id) + '/logs');
      activeSSE.addEventListener('log', (ev) => {
        const span = document.createElement('span'); span.className = 'ln'; span.textContent = ev.data;
        logview.appendChild(span); autoscroll();
      });
      activeSSE.addEventListener('end', () => { activeSSE.close(); activeSSE = null; setTimeout(() => jobDetail(id), 600); });
      activeSSE.onerror = () => { if (activeSSE) { activeSSE.close(); activeSSE = null; } };
    }
  }
  async function cancelJob(id) {
    try { await API.cancelJob(id); UI.toast('Abbruch angefordert', 'ok'); }
    catch (e) { UI.apiError(e); }
  }

  // ---------- Settings ----------
  async function settings() {
    set('<div class="loading">Lade…</div>');
    let s;
    try { s = await API.settings(); } catch (e) { return UI.apiError(e); }
    const html = `
      <div class="alert alert-info alert-inline">Einstellungen sind zur Laufzeit <strong>read-only</strong>. Änderungen in <span class="tag">config.yaml</span> vornehmen und Dienst neu starten.</div>
      <div class="grid grid-2">
        <div class="card"><div class="card-title">Server</div><dl class="kv">
          <dt>Bind</dt><dd class="mono">${UI.esc(s.server.bind)}</dd>
          <dt>Port</dt><dd class="mono">${s.server.port}</dd>
          <dt>Auth-Modus</dt><dd>${s.auth.mode === 'none' ? '<span class="badge badge-yellow">none</span>' : UI.esc(s.auth.mode)}</dd>
          <dt>Open ohne Auth</dt><dd>${s.security.allow_open_without_auth ? 'erlaubt' : 'nein'}</dd>
        </dl></div>
        <div class="card"><div class="card-title">acme.sh</div><dl class="kv">
          <dt>Binary</dt><dd class="mono">${UI.esc(s.acme.binary)}</dd>
          <dt>Home</dt><dd class="mono">${UI.esc(s.acme.home)}</dd>
          <dt>Standard-CA</dt><dd class="mono">${UI.esc(s.acme.default_ca || '–')}</dd>
          <dt>Standard-Key</dt><dd class="mono">${UI.esc(s.acme.default_key_type)}</dd>
        </dl></div>
        <div class="card"><div class="card-title">Jobs</div><dl class="kv">
          <dt>Max parallel</dt><dd>${s.jobs.max_parallel}</dd>
          <dt>Timeout</dt><dd>${s.jobs.timeout_seconds}s</dd>
          <dt>Log-Retention</dt><dd>${s.jobs.log_retention_days} Tage</dd>
          <dt>Bald-ab Schwelle</dt><dd>${s.certs.expiring_soon_days} Tage</dd>
        </dl></div>
        <div class="card"><div class="card-title">Reload-Vorlagen</div>
          ${(s.reload_commands || []).length ? '<dl class="kv">' + s.reload_commands.map(r => `<dt>${UI.esc(r.name)}</dt><dd class="mono">${UI.esc((r.command || []).join(' '))}</dd>`).join('') + '</dl>' : '<div class="muted">keine</div>'}
        </div>
      </div>`;
    set(html);
  }

  // ---------- System ----------
  async function system() {
    set('<div class="loading">Lade…</div>');
    let s;
    try { s = await API.system(); } catch (e) { return UI.apiError(e); }
    const yn = (b) => b ? '<span class="badge badge-green dot">ok</span>' : '<span class="badge badge-red dot">nein</span>';
    const html = `<div class="grid grid-2">
      <div class="card"><div class="card-title">acme.sh</div><dl class="kv">
        <dt>Binary</dt><dd class="mono">${UI.esc(s.acme_binary)}</dd>
        <dt>Gefunden</dt><dd>${yn(s.acme_found)}</dd>
        <dt>Ausführbar</dt><dd>${yn(s.acme_exec)}</dd>
        <dt>Version</dt><dd class="mono">${UI.esc(s.acme_version || '–')}</dd>
        <dt>Home</dt><dd class="mono">${UI.esc(s.acme_home)}</dd>
        <dt>Home lesbar</dt><dd>${yn(s.home_readable)}</dd>
      </dl></div>
      <div class="card"><div class="card-title">acmesh-ui</div><dl class="kv">
        <dt>Version</dt><dd class="mono">${UI.esc(s.ui_version)}</dd>
        <dt>Datenverzeichnis</dt><dd class="mono">${UI.esc(s.data_dir)}</dd>
        <dt>Schreibbar</dt><dd>${yn(s.data_writable)}</dd>
        <dt>Config</dt><dd class="mono">${UI.esc(s.config_path || '–')}</dd>
      </dl></div>
      <div class="card" id="update-card"><div class="card-title">Updates</div>
        <div id="update-body"><div class="muted">Suche nach Updates…</div></div>
      </div>
      <div class="card" style="grid-column:1/-1"><div class="card-title">Renewal-Mechanismen</div>
        <ul>${(s.renewal_hints || []).map(h => `<li class="muted">${UI.esc(h)}</li>`).join('')}</ul>
      </div></div>`;
    set(html);
    loadUpdate();
  }

  async function loadUpdate() {
    const body = document.getElementById('update-body');
    if (!body) return;
    let u;
    try { u = await API.updateCheck(); }
    catch (e) { body.innerHTML = `<div class="muted">Update-Prüfung nicht möglich: ${UI.esc((e && e.message) || 'Fehler')}</div>`; return; }

    let html = `<dl class="kv">
      <dt>Installiert</dt><dd class="mono">${UI.esc(u.current)}</dd>
      <dt>Verfügbar</dt><dd class="mono">${UI.esc(u.latest || '–')}</dd>
    </dl>`;
    if (u.update_available) {
      html += `<div class="alert alert-info alert-inline" style="margin-top:12px">Neue Version <span class="tag">${UI.esc(u.latest)}</span> verfügbar.</div>`;
      if (u.can_self_update) {
        html += `<button class="btn btn-primary" id="update-btn">Auf ${UI.esc(u.latest)} aktualisieren</button>`;
      } else {
        html += `<div class="alert alert-warn alert-inline">${UI.esc(u.note || 'Selbst-Update nicht möglich (Schreibrechte fehlen).')} Auf dem Host: <span class="tag">sudo acmesh-ui update</span></div>`;
      }
    } else {
      html += `<div class="alert alert-info alert-inline" style="margin-top:12px">✓ Aktuell – kein Update verfügbar.</div>`;
    }
    body.innerHTML = html;
    const btn = document.getElementById('update-btn');
    if (btn) btn.onclick = () => confirmUpdate(u);
  }

  function confirmUpdate(u) {
    UI.modal({
      title: 'Update installieren',
      confirmLabel: `Auf ${u.latest} aktualisieren`,
      bodyHtml: `<p>Aktualisiert acmesh-ui von <span class="tag">${UI.esc(u.current)}</span> auf <span class="tag">${UI.esc(u.latest)}</span>.</p>
        <p class="muted">Die Binary wird heruntergeladen, per SHA-256-Checksum geprüft und ersetzt.
        ${u.restart_supported ? 'Anschließend startet die Anwendung neu und diese Seite lädt automatisch neu.' : 'Danach muss der Dienst manuell neu gestartet werden.'}</p>`,
      onConfirm: async () => {
        const r = await API.updateApply();
        UI.closeModal();
        if (r.restarting) waitForRestart(r.version);
        else UI.toast('Update installiert (' + r.version + ') – Dienst manuell neu starten', 'ok');
      },
    });
  }

  // Overlay shown while the server re-execs; polls /api/status and reloads once
  // the new version answers.
  function waitForRestart(targetVersion) {
    const ov = document.createElement('div');
    ov.className = 'modal-overlay';
    ov.style.display = 'flex';
    ov.innerHTML = `<div class="modal" style="max-width:420px;text-align:center">
      <div class="modal-body">
        <div style="font-size:34px">⏳</div>
        <h3 style="margin:.4em 0">Anwendung wird aktualisiert…</h3>
        <p class="muted" id="restart-msg">Neue Version <span class="tag">${UI.esc(targetVersion)}</span> wird gestartet. Bitte warten.</p>
      </div></div>`;
    document.body.appendChild(ov);

    const start = Date.now();
    const tick = async () => {
      if (Date.now() - start > 120000) {
        document.getElementById('restart-msg').innerHTML = 'Zeitüberschreitung. Bitte Seite manuell neu laden.';
        return;
      }
      try {
        const s = await API.status();
        // Server is back. If we know the version, wait until it matches; else reload.
        if (!targetVersion || s.ui_version === targetVersion || s.ui_version !== undefined) {
          document.getElementById('restart-msg').textContent = 'Fertig – lade neu…';
          setTimeout(() => location.reload(), 600);
          return;
        }
      } catch (e) { /* server still down, keep polling */ }
      setTimeout(tick, 1500);
    };
    // Give the server a moment to drop the old listener before polling.
    setTimeout(tick, 2000);
  }

  return { newCert, dns, dnsEdit, dnsDelete, jobs, jobDetail, cancelJob, settings, system };
})());
