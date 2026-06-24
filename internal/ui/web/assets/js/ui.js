// Shared UI helpers: escaping, toasts, modals, badges, formatting.
const UI = (() => {
  function esc(s) {
    if (s === null || s === undefined) return '';
    return String(s).replace(/[&<>"']/g, (c) => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    }[c]));
  }

  function toast(msg, kind) {
    const el = document.createElement('div');
    el.className = 'toast ' + (kind || '');
    el.textContent = msg;
    document.getElementById('toasts').appendChild(el);
    setTimeout(() => { el.style.opacity = '0'; setTimeout(() => el.remove(), 250); }, 4200);
  }

  function apiError(e) {
    const msg = (e && e.message) || 'Unbekannter Fehler';
    const det = (e && e.details) ? (' — ' + e.details) : '';
    toast(msg + det, 'err');
  }

  // confirm modal. opts: {title, bodyHtml, confirmLabel, danger, onConfirm}
  function modal(opts) {
    const overlay = document.getElementById('modal-overlay');
    document.getElementById('modal-title').textContent = opts.title || 'Bestätigen';
    document.getElementById('modal-body').innerHTML = opts.bodyHtml || '';
    const foot = document.getElementById('modal-foot');
    foot.innerHTML = '';
    const cancel = document.createElement('button');
    cancel.className = 'btn btn-ghost';
    cancel.textContent = 'Abbrechen';
    cancel.onclick = closeModal;
    const ok = document.createElement('button');
    ok.className = 'btn ' + (opts.danger ? 'btn-danger' : 'btn-primary');
    ok.textContent = opts.confirmLabel || 'Bestätigen';
    ok.onclick = async () => {
      ok.disabled = true;
      try { await opts.onConfirm(); closeModal(); }
      catch (e) { apiError(e); ok.disabled = false; }
    };
    foot.appendChild(cancel);
    foot.appendChild(ok);
    overlay.style.display = 'flex';
    if (opts.onRender) opts.onRender(document.getElementById('modal-body'));
  }
  function closeModal() { document.getElementById('modal-overlay').style.display = 'none'; }

  function statusBadge(status) {
    const map = {
      valid: ['badge-green', 'gültig'], expiring: ['badge-yellow', 'läuft bald ab'],
      expired: ['badge-red', 'abgelaufen'], error: ['badge-red', 'fehlerhaft'],
      unknown: ['badge-gray', 'unbekannt'],
    };
    const m = map[status] || ['badge-gray', status];
    return `<span class="badge ${m[0]} dot">${esc(m[1])}</span>`;
  }

  function jobBadge(status) {
    const map = {
      queued: ['badge-gray', 'wartet'], running: ['badge-blue', 'läuft'],
      success: ['badge-green', 'erfolgreich'], failed: ['badge-red', 'fehlgeschlagen'],
      cancelled: ['badge-gray', 'abgebrochen'],
    };
    const m = map[status] || ['badge-gray', status];
    return `<span class="badge ${m[0]} dot">${esc(m[1])}</span>`;
  }

  function relDays(days) {
    if (days === null || days === undefined) return '';
    if (days < 0) return `vor ${Math.abs(days)} Tagen abgelaufen`;
    if (days === 0) return 'läuft heute ab';
    return `läuft in ${days} Tagen ab`;
  }

  function fmtDate(s) {
    if (!s || s.startsWith('0001')) return '–';
    const d = new Date(s);
    if (isNaN(d)) return '–';
    return d.toLocaleString('de-DE', { dateStyle: 'medium', timeStyle: 'short' });
  }

  function fmtDuration(ns) {
    if (!ns) return '–';
    const sec = Math.round(ns / 1e9);
    if (sec < 60) return sec + 's';
    return Math.floor(sec / 60) + 'm ' + (sec % 60) + 's';
  }

  function copyBtn(text) {
    return `<button class="btn btn-sm copy-btn" onclick="UI.copy(this, ${JSON.stringify(text).replace(/"/g, '&quot;')})">Kopieren</button>`;
  }
  function copy(btn, text) {
    navigator.clipboard.writeText(text).then(() => {
      const old = btn.textContent; btn.textContent = '✓ Kopiert';
      setTimeout(() => btn.textContent = old, 1500);
    }).catch(() => toast('Kopieren fehlgeschlagen', 'err'));
  }

  function codeblock(text, withCopy) {
    return `<div class="codeblock">${withCopy ? copyBtn(text) : ''}${esc(text)}</div>`;
  }

  return { esc, toast, apiError, modal, closeModal, statusBadge, jobBadge, relDays, fmtDate, fmtDuration, copy, copyBtn, codeblock };
})();
