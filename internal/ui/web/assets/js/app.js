// Router, theme, global chrome.
(() => {
  const routes = {
    dashboard: { title: 'Dashboard', render: () => Pages.dashboard() },
    certs: { title: 'Zertifikate', render: () => Pages.certs() },
    certDetail: { title: 'Zertifikat', render: (id) => Pages.certDetail(id) },
    new: { title: 'Neues Zertifikat', render: () => Pages.newCert() },
    dns: { title: 'DNS-Provider', render: () => Pages.dns() },
    jobs: { title: 'Jobs', render: () => Pages.jobs() },
    jobDetail: { title: 'Job', render: (id) => Pages.jobDetail(id) },
    settings: { title: 'Einstellungen', render: () => Pages.settings() },
    system: { title: 'Systemstatus', render: () => Pages.system() },
  };

  function parseHash() {
    const h = (location.hash || '#/dashboard').replace(/^#\//, '');
    const parts = h.split('/');
    if (parts[0] === 'certs' && parts[1]) return { route: 'certDetail', nav: 'certs', arg: decodeURIComponent(parts[1]) };
    if (parts[0] === 'jobs' && parts[1]) return { route: 'jobDetail', nav: 'jobs', arg: decodeURIComponent(parts[1]) };
    const name = parts[0] || 'dashboard';
    return { route: routes[name] ? name : 'dashboard', nav: name, arg: null };
  }

  function navigate() {
    const { route, nav, arg } = parseHash();
    const r = routes[route];
    document.getElementById('page-title').textContent = r.title;
    document.querySelectorAll('#nav a').forEach(a => a.classList.toggle('active', a.dataset.route === nav));
    document.getElementById('sidebar').classList.remove('open');
    r.render(arg);
  }

  // Theme
  function applyTheme(t) {
    document.documentElement.setAttribute('data-theme', t);
    localStorage.setItem('acmesh-theme', t);
    document.getElementById('theme-toggle').textContent = t === 'dark' ? '🌙' : '☀';
  }
  function initTheme() {
    applyTheme(localStorage.getItem('acmesh-theme') || 'dark');
    document.getElementById('theme-toggle').onclick = () => {
      const cur = document.documentElement.getAttribute('data-theme');
      applyTheme(cur === 'dark' ? 'light' : 'dark');
    };
  }

  // Global status (header indicators, warnings, footer)
  async function refreshStatus() {
    try {
      const s = await API.status();
      document.getElementById('brand-title').textContent = s.title || 'acmesh-ui';
      document.getElementById('foot-ui-version').textContent = s.ui_version;
      document.getElementById('foot-acme-version').textContent = s.acme_version || '–';
      document.getElementById('footer-version').textContent = s.ui_version;

      const acmeInd = document.getElementById('acme-indicator');
      acmeInd.className = 'badge ' + (s.acme_found ? 'badge-green dot' : 'badge-red dot');
      acmeInd.textContent = s.acme_found ? 'acme.sh ' + (s.acme_version || '') : 'acme.sh fehlt';

      const authInd = document.getElementById('auth-indicator');
      const warn = document.getElementById('auth-warning');
      if (s.auth_disabled) {
        authInd.className = 'badge badge-yellow dot'; authInd.textContent = 'Auth: aus';
        warn.style.display = 'block';
        document.getElementById('auth-warning-text').textContent =
          ' Zugriff nur über VPN / SSH-Tunnel / Reverse-Proxy absichern.' +
          (s.open_bind ? ' ACHTUNG: an ' + s.bind + ' gebunden (netzwerkweit erreichbar)!' : '');
        document.getElementById('footer-auth').textContent = 'Auth deaktiviert';
      } else {
        authInd.className = 'badge badge-green dot'; authInd.textContent = 'Auth: ' + s.auth_mode;
        warn.style.display = 'none';
        document.getElementById('footer-auth').textContent = '';
      }
    } catch (e) { /* status endpoint unreachable - leave defaults */ }
  }

  function init() {
    initTheme();
    document.getElementById('sidebar-toggle').onclick = () => document.getElementById('sidebar').classList.toggle('open');
    document.getElementById('modal-close').onclick = UI.closeModal;
    document.getElementById('modal-overlay').onclick = (e) => { if (e.target.id === 'modal-overlay') UI.closeModal(); };
    document.getElementById('refresh-btn').onclick = async () => {
      try { await API.scan(); } catch (e) {}
      await refreshStatus(); navigate(); UI.toast('Aktualisiert', 'ok');
    };
    window.addEventListener('hashchange', navigate);
    refreshStatus();
    setInterval(refreshStatus, 30000);
    navigate();
  }

  document.addEventListener('DOMContentLoaded', init);
})();
