self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()));
self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;
  const url = new URL(event.request.url);
  if (url.pathname === '/v1/incidents') {
    return event.respondWith(handleIncidents());
  }
  if (url.pathname === '/v1/hosts') {
    return event.respondWith(handleHosts());
  }
});
// /v1/hosts: Zabbix host.get の result を返す
async function handleHosts() {
  const hostsResp = await zabbixApiRequest('host.get', {
    output: ['hostid', 'host', 'name', 'status', 'ip'],
  });
  const hosts = hostsResp.result || [];
  return new Response(JSON.stringify(hosts), {
    status: 200,
    headers: { 'Content-Type': 'application/json' }
  });
}

const ZABBIX_API_URL = '/zabbix/api_jsonrpc.php';
async function handleIncidents() {
  // problem.getでeventid一覧のみ取得
  const problemsResp = await zabbixApiRequest('problem.get', {
    output: ['eventid'],
    suppressed: false
  });
  const problems = problemsResp.result || [];
  const eventids = problems.map(p => p.eventid).filter(Boolean);
  if (eventids.length === 0) {
    return new Response(JSON.stringify([]), {
      status: 200,
      headers: { 'Content-Type': 'application/json' }
    });
  }

  // event.getで詳細・hostsまとめて取得
  const eventsResp = await zabbixApiRequest('event.get', {
    output: ['eventid', 'clock', 'name'],
    eventids: eventids,
    selectHosts: ['hostid', 'host']
  });
  const events = (eventsResp.result || []).map(ev => {
    // clock(UNIXタイム)→RFC3339(ISO8601)文字列へ変換
    let clockIso = '';
    if (ev.clock) {
      const d = new Date(Number(ev.clock) * 1000);
      clockIso = d.toISOString();
    }
    return {
      eventid: ev.eventid,
      clock: clockIso,
      name: ev.name,
      hosts: Array.isArray(ev.hosts) ? ev.hosts : []
    };
  });
  return new Response(JSON.stringify(events), {
    status: 200,
    headers: { 'Content-Type': 'application/json' }
  });
}

// Zabbix APIを呼び出す共通関数
async function zabbixApiRequest(method, params) {
  const payload = {
    jsonrpc: '2.0',
    method,
    params,
    id: 1
  };
  const apiResp = await fetch(ZABBIX_API_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
    credentials: 'same-origin'
  });
  return await apiResp.json();
}
