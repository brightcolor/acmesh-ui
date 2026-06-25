// Page renderers. Each renders into #content.
const Pages = (() => {
  const el = () => document.getElementById('content');
  const set = (html) => { el().innerHTML = html; };
  const loading = () => set('<div class="loading">Lade…</div>');

  // ---------- Dashboard ----------
  async function dashboard() {
    loading();
    let d;
    try { d = await API.dashboard(); } catch (e) { return UI.apiError(e); }
    const s = d.status;
    const stat = (num, label, hint, cls, ico) => `
      <div class="stat accent-${cls}"><span class="stat-ico">${ico}</span>
        <div class="stat-num">${num}</div><div class="stat-label">${label}</div>
        <div class="stat-hint">${hint || ''}</div></div>`;
    let html = `<div class="grid grid-stat">
      ${stat(d.total, 'Zertifikate gesamt', 'im acme.sh Home', 'blue', '🛡')}
      ${stat(d.valid, 'Gültig', '', 'green', '✓')}
      ${stat(d.expiring, 'Laufen bald ab', '< ' + d.expiring_days + ' Tage', 'yellow', '⏰')}
      ${stat(d.expired, 'Abgelaufen', '', 'red', '✕')}
      ${stat(d.failed_jobs, 'Fehlgeschlagene Jobs', '', 'red', '⚙')}
    </div>`;

    html += `<div class="grid grid-2" style="margin-top:16px">
      <div class="card"><div class="card-title">acme.sh Systemstatus</div>
        <dl class="kv">
          <dt>Status</dt><dd>${s.acme_found ? '<span class="badge badge-green dot">erreichbar</span>' : '<span class="badge badge-red dot">nicht gefunden</span>'}</dd>
          <dt>Version</dt><dd class="mono">${UI.esc(s.acme_version || '–')}</dd>
          <dt>Pfad</dt><dd class="mono">${UI.esc(s.acme_path)}</dd>
          <dt>Home</dt><dd class="mono">${UI.esc(s.acme_home)}</dd>
          <dt>WebUI</dt><dd class="mono">${UI.esc(s.ui_version)}</dd>
          <dt>Auth</dt><dd>${s.auth_disabled ? '<span class="badge badge-yellow dot">deaktiviert</span>' : '<span class="badge badge-green dot">' + UI.esc(s.auth_mode) + '</span>'}</dd>
        </dl>
      </div>
      <div class="card"><div class="card-title">Nächste kritische Abläufe</div>
        ${renderExpiringList(d.expiring_soon)}
      </div>
    </div>`;

    html += `<div class="card" style="margin-top:16px"><div class="card-title">Letzte Jobs</div>${renderRecentJobs(d.recent_jobs)}</div>`;
    set(html);
  }

  function renderExpiringList(list) {
    if (!list || !list.length) return '<div class="empty">Keine bald ablaufenden Zertifikate 🎉</div>';
    return '<div class="table-wrap"><table class="tbl"><tbody>' + list.map(c => `
      <tr><td><a href="#/certs/${UI.esc(c.id)}" class="mono">${UI.esc(c.main_domain)}</a></td>
      <td>${UI.statusBadge(c.status)}</td>
      <td class="muted nowrap">${UI.esc(UI.relDays(c.days_remaining))}</td></tr>`).join('') + '</tbody></table></div>';
  }

  function renderRecentJobs(jobs) {
    if (!jobs || !jobs.length) return '<div class="empty">Noch keine Jobs ausgeführt.</div>';
    return '<div class="table-wrap"><table class="tbl"><thead><tr><th>Typ</th><th>Domain</th><th>Status</th><th>Start</th></tr></thead><tbody>' +
      jobs.map(j => `<tr><td><a href="#/jobs/${UI.esc(j.id)}">${UI.esc(j.type)}</a></td>
        <td class="mono">${UI.esc(j.domain || '–')}</td><td>${UI.jobBadge(j.status)}</td>
        <td class="muted nowrap">${UI.fmtDate(j.queued_at)}</td></tr>`).join('') +
      '</tbody></table></div>';
  }

  // ---------- Certificates ----------
  let certState = { all: [], filter: '', search: '' };
  async function certs() {
    loading();
    let d;
    try { d = await API.certs(); } catch (e) { return UI.apiError(e); }
    certState.all = d.certs || [];
    renderCerts();
  }
  function renderCerts() {
    const filters = ['', 'valid', 'expiring', 'expired', 'error'];
    const labels = { '': 'Alle', valid: 'Gültig', expiring: 'Bald ab', expired: 'Abgelaufen', error: 'Fehler' };
    let list = certState.all.filter(c =>
      (!certState.filter || c.status === certState.filter) &&
      (!certState.search || (c.main_domain + ' ' + (c.sans || []).join(' ')).toLowerCase().includes(certState.search.toLowerCase())));
    list = list.slice().sort((a, b) => (a.days_remaining ?? 1e9) - (b.days_remaining ?? 1e9));

    let html = `<div class="page-head">
      <div class="toolbar">
        <input type="text" id="cert-search" placeholder="Domain suchen…" value="${UI.esc(certState.search)}">
        <div class="filterchips">${filters.map(f => `<span class="chip ${certState.filter === f ? 'active' : ''}" data-f="${f}">${labels[f]}</span>`).join('')}</div>
      </div>
      <button class="btn btn-primary" onclick="location.hash='#/new'">＋ Neues Zertifikat</button>
    </div>`;

    if (!list.length) {
      html += '<div class="card"><div class="empty"><div class="big">🛡</div>Keine Zertifikate gefunden.</div></div>';
    } else {
      html += `<div class="card"><div class="table-wrap"><table class="tbl">
        <thead><tr><th>Domain</th><th>SANs</th><th>Status</th><th>Ablauf</th><th>Key</th><th>Aktionen</th></tr></thead><tbody>` +
        list.map(certRow).join('') + '</tbody></table></div></div>';
    }
    set(html);

    document.getElementById('cert-search').addEventListener('input', (e) => { certState.search = e.target.value; renderCerts(); });
    document.querySelectorAll('.chip[data-f]').forEach(ch => ch.onclick = () => { certState.filter = ch.dataset.f; renderCerts(); });
  }
  function certRow(c) {
    const sans = (c.sans || []).filter(s => s !== c.main_domain);
    return `<tr>
      <td><a href="#/certs/${UI.esc(c.id)}" class="mono">${UI.esc(c.main_domain)}</a> ${c.wildcard ? '<span class="tag">wildcard</span>' : ''}</td>
      <td class="muted">${sans.length ? UI.esc(sans.slice(0, 2).join(', ')) + (sans.length > 2 ? ` +${sans.length - 2}` : '') : '–'}</td>
      <td>${UI.statusBadge(c.status)}</td>
      <td class="nowrap">${UI.fmtDate(c.not_after).split(',')[0]}<div class="muted" style="font-size:11px">${UI.esc(UI.relDays(c.days_remaining))}</div></td>
      <td class="mono">${UI.esc(c.key_type || '–')}</td>
      <td><div class="row-actions">
        <a class="btn btn-sm" href="#/certs/${UI.esc(c.id)}">Details</a>
        <button class="btn btn-sm" onclick="Pages.reissue('${UI.esc(c.id)}')">Edit</button>
        <button class="btn btn-sm" onclick="Pages.confirmRenew('${UI.esc(c.id)}', false)">Renew</button>
        <button class="btn btn-sm btn-danger" onclick="Pages.confirmDelete('${UI.esc(c.id)}','${UI.esc(c.main_domain)}')">Löschen</button>
      </div></td></tr>`;
  }

  async function confirmRenew(id, force) {
    UI.modal({
      title: force ? 'Force Renew starten' : 'Renew starten',
      bodyHtml: `<p>${force ? 'Erzwinge die Erneuerung' : 'Erneuere'} das Zertifikat <span class="tag">${UI.esc(id)}</span>.</p>
        ${force ? '<div class="alert alert-danger alert-inline">Force Renew ignoriert die Restlaufzeit und kann CA-Rate-Limits verbrauchen.</div>' : ''}
        <p class="muted">Die Ausführung läuft als Hintergrundjob. Das Ergebnis erscheint unter Jobs.</p>`,
      confirmLabel: force ? 'Force Renew starten' : 'Renew starten',
      danger: force,
      onConfirm: async () => {
        const r = force ? await API.forceRenew(id) : await API.renew(id);
        UI.toast('Job gestartet', 'ok');
        location.hash = '#/jobs/' + r.job_id;
      },
    });
  }

  // ---------- Cert detail ----------
  async function certDetail(id) {
    loading();
    let d;
    try { d = await API.cert(id); } catch (e) { return UI.apiError(e); }
    const c = d.cert;
    const paths = [['Cert', c.cert_path], ['Key', c.key_path], ['Fullchain', c.fullchain_path], ['CA/Chain', c.ca_path], ['Conf', c.conf_path]]
      .filter(p => p[1]);
    let html = `<div class="page-head"><h2 class="mono">${UI.esc(c.main_domain)} ${UI.statusBadge(c.status)}</h2>
      <div class="btn-row">
        <button class="btn" onclick="Pages.reissue('${UI.esc(c.id)}')">Bearbeiten / Re-Issue</button>
        <button class="btn" onclick="Pages.confirmRenew('${UI.esc(c.id)}', false)">Renew</button>
        <button class="btn" onclick="Pages.confirmRenew('${UI.esc(c.id)}', true)">Force Renew</button>
        <button class="btn" onclick="Pages.installModal('${UI.esc(c.id)}')">Install</button>
        <button class="btn" onclick="Pages.deployModal('${UI.esc(c.id)}')">Deploy</button>
        <button class="btn btn-danger" onclick="Pages.confirmDelete('${UI.esc(c.id)}','${UI.esc(c.main_domain)}')">Löschen</button>
      </div></div>`;
    if (c.parse_error) html += `<div class="alert alert-danger alert-inline">Parse-Fehler: ${UI.esc(c.parse_error)}</div>`;
    html += `<div class="grid grid-2">
      <div class="card"><div class="card-title">Übersicht</div><dl class="kv">
        <dt>Hauptdomain</dt><dd class="mono">${UI.esc(c.main_domain)}</dd>
        <dt>Wildcard</dt><dd>${c.wildcard ? 'ja' : 'nein'}</dd>
        <dt>Status</dt><dd>${UI.statusBadge(c.status)} <span class="muted">${UI.esc(UI.relDays(c.days_remaining))}</span></dd>
        <dt>Gültig ab</dt><dd>${UI.fmtDate(c.not_before)}</dd>
        <dt>Gültig bis</dt><dd>${UI.fmtDate(c.not_after)}</dd>
        ${c.next_renew && !c.next_renew.startsWith('0001') ? `<dt>Nächster Renewal</dt><dd>${UI.fmtDate(c.next_renew)} <span class="muted">(acme.sh)</span></dd>` : ''}
        <dt>Issuer</dt><dd>${UI.esc(c.issuer || '–')}</dd>
        <dt>Key-Typ</dt><dd class="mono">${UI.esc(c.key_type || '–')}</dd>
        <dt>CA</dt><dd class="mono">${UI.esc(c.ca || '–')}</dd>
        <dt>Serial</dt><dd class="mono">${UI.esc(c.serial || '–')}</dd>
        <dt>Fingerprint</dt><dd class="mono" style="font-size:11px">${UI.esc(c.fingerprint || '–')}</dd>
      </dl></div>
      <div class="card"><div class="card-title">SANs</div>
        ${(c.sans || []).length ? '<div>' + c.sans.map(s => `<span class="tag" style="margin:2px">${UI.esc(s)}</span>`).join(' ') + '</div>' : '<div class="muted">–</div>'}
        <div class="card-title" style="margin-top:18px">Pfade</div>
        <dl class="kv">${paths.map(p => `<dt>${p[0]}</dt><dd class="mono">${UI.esc(p[1])}</dd>`).join('')}</dl>
        ${c.install ? renderInstallConf(c.install) : ''}
      </div></div>`;
    // Live TLS endpoint check + files cards.
    html += `<div class="grid grid-2">
      <div class="card"><div class="page-head"><div class="card-title">Live-Endpoint (TLS)</div>
        <button class="btn btn-sm" id="tls-btn">Prüfen</button></div>
        <div class="muted" id="tls-result">Vergleicht das tatsächlich ausgelieferte Zertifikat unter <span class="tag">${UI.esc(c.main_domain)}:443</span> mit dem ausgestellten.</div>
      </div>
      <div class="card"><div class="card-title">Dateien</div>
        <div class="btn-row">
          ${c.fullchain_path ? `<button class="btn btn-sm" onclick="Pages.viewPem('${UI.esc(c.id)}','fullchain')">Fullchain ansehen</button>` : ''}
          ${c.cert_path ? `<button class="btn btn-sm" onclick="Pages.viewPem('${UI.esc(c.id)}','cert')">Cert ansehen</button>` : ''}
          ${c.fullchain_path ? `<a class="btn btn-sm" href="${API.certDownloadUrl(c.id, 'fullchain')}">Fullchain ⬇</a>` : ''}
          ${c.cert_path ? `<a class="btn btn-sm" href="${API.certDownloadUrl(c.id, 'cert')}">Cert ⬇</a>` : ''}
          ${c.ca_path ? `<a class="btn btn-sm" href="${API.certDownloadUrl(c.id, 'chain')}">Chain ⬇</a>` : ''}
          ${c.key_path ? `<button class="btn btn-sm btn-danger" onclick="Pages.downloadKey('${UI.esc(c.id)}','${UI.esc(c.main_domain)}')">Key ⬇</button>` : ''}
        </div>
        <div id="pem-view" style="margin-top:12px"></div>
      </div>
    </div>`;

    html += `<div class="card"><div class="card-title">Letzte Jobs für diese Domain</div>${renderRecentJobs(d.jobs)}</div>`;
    set(html);

    const tlsBtn = document.getElementById('tls-btn');
    if (tlsBtn) tlsBtn.onclick = () => runTlsCheck(c.id, c.main_domain);
  }

  async function runTlsCheck(id, host) {
    const out = document.getElementById('tls-result');
    out.innerHTML = '<span class="muted">Prüfe…</span>';
    let r;
    try { r = await API.tlsCheck(id); } catch (e) { out.innerHTML = `<span class="badge badge-red dot">Fehler</span> ${UI.esc((e && e.message) || '')}`; return; }
    const s = r.served;
    if (!s.reachable) {
      out.innerHTML = `<div class="alert alert-warn alert-inline">${UI.esc(host)}:443 nicht erreichbar.<div class="muted" style="font-size:12px">${UI.esc(s.error || '')}</div></div>`;
      return;
    }
    const badge = r.match
      ? '<span class="badge badge-green dot">stimmt überein</span>'
      : '<span class="badge badge-red dot">weicht ab</span>';
    out.innerHTML = `<div>${badge} ${r.match ? 'Das ausgelieferte Zertifikat entspricht dem ausgestellten.' : 'Anderes Zertifikat ausgeliefert — evtl. Dienst nicht reloaded oder anderer Host.'}</div>
      <dl class="kv" style="margin-top:10px">
        <dt>Subject</dt><dd class="mono">${UI.esc(s.subject || '–')}</dd>
        <dt>Issuer</dt><dd>${UI.esc(s.issuer || '–')}</dd>
        <dt>Gültig bis</dt><dd>${UI.fmtDate(s.not_after)}</dd>
        <dt>Fingerprint (served)</dt><dd class="mono" style="font-size:11px">${UI.esc(s.fingerprint || '–')}</dd>
      </dl>`;
  }

  async function viewPem(id, file) {
    const view = document.getElementById('pem-view');
    view.innerHTML = '<span class="muted">Lade…</span>';
    try {
      const r = await API.certPem(id, file);
      view.innerHTML = `<div class="card-title">${UI.esc(file)} — ${UI.esc(r.path)}</div>${UI.codeblock(r.pem, true)}`;
    } catch (e) { view.innerHTML = `<span class="badge badge-red dot">Fehler</span> ${UI.esc((e && e.message) || '')}`; }
  }

  function downloadKey(id, domain) {
    UI.modal({
      title: 'Privaten Schlüssel herunterladen', danger: true, confirmLabel: 'Key herunterladen',
      bodyHtml: `<div class="alert alert-danger alert-inline">Der <strong>private Schlüssel</strong> von <span class="tag">${UI.esc(domain)}</span> wird heruntergeladen. Bewahre ihn sicher auf — wer ihn besitzt, kann sich als dieser Host ausgeben.</div>`,
      onConfirm: async () => {
        window.location.href = API.certDownloadUrl(id, 'key', true);
        UI.closeModal();
      },
    });
  }

  async function confirmDelete(id, domain) {
    UI.modal({
      title: 'Zertifikat löschen', danger: true, confirmLabel: 'Zertifikat löschen',
      bodyHtml: `<p>Entfernt <span class="tag">${UI.esc(domain)}</span> aus der acme.sh-Verwaltung (kein Auto-Renewal mehr).</p>
        ${UI.codeblock('acme.sh --remove -d ' + domain)}
        <div class="checkbox" style="margin-top:12px"><input type="checkbox" id="del-purge"><label style="margin:0" for="del-purge">Zertifikatsdateien ebenfalls von der Festplatte löschen</label></div>
        <div class="alert alert-warn alert-inline" style="margin-top:10px">Installierte Kopien (z. B. in /etc/ssl) bleiben unberührt — nur das acme.sh-Verzeichnis wird betroffen.</div>`,
      onConfirm: async () => {
        const purge = document.getElementById('del-purge').checked;
        const r = await API.deleteCert(id, purge);
        UI.toast('Löschen gestartet', 'ok');
        location.hash = '#/jobs/' + r.job_id;
      },
    });
  }

  async function reissue(id) {
    let c;
    try { c = (await API.cert(id)).cert; } catch (e) { return UI.apiError(e); }
    const ri = c.reissue || {};
    const sans = (c.sans || []).filter(s => s !== c.main_domain && !s.startsWith('*.'));
    Pages._prefill = {
      main: c.main_domain,
      sans,
      wildcard: (c.sans || []).some(s => s.startsWith('*.')),
      challenge: ri.challenge || 'dns',
      webroot: ri.webroot || '',
      dnsCode: ri.dns_code || '',
      keyType: ri.key_type || c.key_type || 'ec-256',
      reissueOf: c.main_domain,
    };
    location.hash = '#/new';
  }
  function renderInstallConf(ic) {
    return `<div class="card-title" style="margin-top:18px">Install-Konfiguration</div><dl class="kv">
      ${ic.key_file ? `<dt>Key</dt><dd class="mono">${UI.esc(ic.key_file)}</dd>` : ''}
      ${ic.fullchain_file ? `<dt>Fullchain</dt><dd class="mono">${UI.esc(ic.fullchain_file)}</dd>` : ''}
      ${ic.reload_cmd ? `<dt>Reload</dt><dd class="mono">${UI.esc(ic.reload_cmd)}</dd>` : ''}
    </dl>`;
  }

  // ---------- Install / Deploy modals ----------
  let reloadTemplates = [];
  async function installModal(id) {
    if (!reloadTemplates.length) { try { reloadTemplates = (await API.settings()).reload_commands || []; } catch (e) {} }
    UI.modal({
      title: 'Zertifikat installieren',
      confirmLabel: 'Zertifikat installieren',
      bodyHtml: `
        <div class="form-row"><label>Key-Datei</label><input type="text" id="i-key" placeholder="/etc/ssl/${UI.esc(id)}/key.pem"></div>
        <div class="form-row"><label>Fullchain-Datei</label><input type="text" id="i-full" placeholder="/etc/ssl/${UI.esc(id)}/fullchain.pem"></div>
        <div class="form-row"><label>Reload-Kommando (Vorlage)</label>
          <select id="i-reload"><option value="">— kein Reload —</option>${reloadTemplates.map(t => `<option value="${UI.esc(t.name)}">${UI.esc(t.name)} (${UI.esc((t.command || []).join(' '))})</option>`).join('')}</select>
          <div class="help">Nur freigegebene Vorlagen aus der config.yaml. Keine freie Shell.</div>
        </div>
        <div id="i-preview"></div>`,
      onConfirm: async () => {
        const body = installBody(id);
        const r = await API.install(id, body);
        UI.toast('Install-Job gestartet', 'ok');
        location.hash = '#/jobs/' + r.job_id;
      },
    });
  }
  function installBody(id) {
    return {
      key_file: document.getElementById('i-key').value.trim(),
      fullchain_file: document.getElementById('i-full').value.trim(),
      reload_name: document.getElementById('i-reload').value,
    };
  }
  async function deployModal(id) {
    UI.modal({
      title: 'Deploy-Hook ausführen',
      confirmLabel: 'Deploy-Hook ausführen',
      bodyHtml: `
        <div class="form-row"><label>Deploy-Hook</label><input type="text" id="d-hook" placeholder="haproxy">
          <div class="help">acme.sh Deploy-Hook-Name (z. B. haproxy, ssh, docker).</div></div>
        <div class="alert alert-info alert-inline">Es wird ausschließlich <span class="tag">acme.sh --deploy</span> mit dem angegebenen Hook ausgeführt.</div>`,
      onConfirm: async () => {
        const r = await API.deploy(id, { hook: document.getElementById('d-hook').value.trim() });
        UI.toast('Deploy-Job gestartet', 'ok');
        location.hash = '#/jobs/' + r.job_id;
      },
    });
  }

  return {
    dashboard, certs, certDetail, confirmRenew, installModal, deployModal,
    confirmDelete, reissue, viewPem, downloadKey,
    _prefill: null,
    // extended in pages2.js
  };
})();
