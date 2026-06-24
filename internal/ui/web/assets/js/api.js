// Minimal JSON API client for acmesh-ui.
const API = (() => {
  async function req(method, path, body) {
    const opts = { method, headers: {} };
    if (body !== undefined) {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(body);
    }
    const res = await fetch(path, opts);
    const text = await res.text();
    let data = null;
    if (text) {
      try { data = JSON.parse(text); } catch (e) { data = { raw: text }; }
    }
    if (!res.ok) {
      const err = (data && data.error) || { code: 'HTTP_' + res.status, message: 'Request failed (' + res.status + ')' };
      throw err;
    }
    return data;
  }

  return {
    get: (p) => req('GET', p),
    post: (p, b) => req('POST', p, b),
    put: (p, b) => req('PUT', p, b),
    del: (p) => req('DELETE', p),

    status: () => req('GET', '/api/status'),
    dashboard: () => req('GET', '/api/dashboard'),
    system: () => req('GET', '/api/system'),
    scan: () => req('POST', '/api/scan'),
    certs: (refresh) => req('GET', '/api/certs' + (refresh ? '?refresh=1' : '')),
    cert: (id) => req('GET', '/api/certs/' + encodeURIComponent(id)),
    issue: (body) => req('POST', '/api/certs', body),
    renew: (id) => req('POST', '/api/certs/' + encodeURIComponent(id) + '/renew'),
    forceRenew: (id) => req('POST', '/api/certs/' + encodeURIComponent(id) + '/force-renew'),
    renewAll: () => req('POST', '/api/certs/renew-all'),
    install: (id, body) => req('POST', '/api/certs/' + encodeURIComponent(id) + '/install', body),
    deploy: (id, body) => req('POST', '/api/certs/' + encodeURIComponent(id) + '/deploy', body),
    jobs: (q) => req('GET', '/api/jobs' + (q || '')),
    job: (id) => req('GET', '/api/jobs/' + encodeURIComponent(id)),
    cancelJob: (id) => req('POST', '/api/jobs/' + encodeURIComponent(id) + '/cancel'),
    dnsProviders: () => req('GET', '/api/dns-providers'),
    dnsProvider: (id) => req('GET', '/api/dns-providers/' + encodeURIComponent(id)),
    createDNS: (body) => req('POST', '/api/dns-providers', body),
    updateDNS: (id, body) => req('PUT', '/api/dns-providers/' + encodeURIComponent(id), body),
    deleteDNS: (id) => req('DELETE', '/api/dns-providers/' + encodeURIComponent(id)),
    settings: () => req('GET', '/api/settings'),
    updateCheck: () => req('GET', '/api/update/check'),
    updateApply: () => req('POST', '/api/update/apply'),
  };
})();
