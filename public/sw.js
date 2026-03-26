/// <reference lib="webworker" />
import * as zabbixApi from './zabbix.js'

/** @type {ServiceWorkerGlobalScope} */
const self = globalThis
let isDemo = false

self.addEventListener('install', () => self.skipWaiting())
self.addEventListener('activate', (ev) => ev.waitUntil(self.clients.claim()))
self.addEventListener('fetch', async (/** @type {FetchEvent} */ ev) => {
  if (ev.request.method !== 'GET') return
  const url = new URL(ev.request.url)
  if (url.origin !== self.location.origin) return
  // メイン
  if (url.pathname === '/v1/incidents') return ev.respondWith(handleIncidents())
  if (url.pathname === '/v1/hosts')
    return ev.respondWith(getHosts().then(v => Response.json(v)))
  if (url.pathname === '/v1/sites')
    return ev.respondWith(getSites().then(v => Response.json(v)))
  if (url.pathname === '/v1/status')
    return ev.respondWith(Response.json({ zabbix: !isDemo }))

  // 個別
  if (url.pathname.startsWith('/zabbix/')) switch (url.pathname) {
    case '/zabbix/problems':
      return ev.respondWith(zabbixApi.getProblems().then(v => Response.json(v)))
    case '/zabbix/events':
      return ev.respondWith(zabbixApi.getEvents().then(v => Response.json(v)))
    case '/zabbix/hosts':
      return ev.respondWith(zabbixApi.getHosts().then(v => Response.json(v)))
  }
})

// ホスト一覧
async function getHosts() {
  const hosts = await zabbixApi.getHosts()
  for (const h of hosts) h.zabbix = true

  // join ciritcal sites
  const r = await fetch('/hands/critical.json')
  /** @type {{}[]} */
  const criticals = await r.json()
  for (const s of criticals) {
    for (const h of hosts.filter(h => h.host.startsWith(s.host))) {
      h.critical = true
    }
  }
  return hosts
}

// サイト一覧
async function getSites() {
  /** @type {Map<string, { host: string, zabbix?: boolean, critical?: boolean }>} */
  const sites = new Map()
  const hosts = await zabbixApi.getHosts()
  for (const h of hosts) {
    const site = h.host.slice(0, 8)
    if (sites.has(site)) continue
    sites.set(site, { zabbix: true })
  }

  // join ciritcal sites
  const r = await fetch('/hands/critical.json')
  /** @type {{}[]} */
  const criticals = await r.json()
  for (const s of criticals) {
    const site = sites.get(s.host) || {}
    sites.set(s.host, { ...site, critical: true })
  }
  return Array.from(sites.entries()).map(([host, data]) => ({ ...data, host }))
}

// // /v1/hosts: Zabbix host.get の result を返す

// const ZABBIX_API_URL = '/zabbix/api_jsonrpc.php';
// async function handleIncidents() {
//   // problem.getでeventid一覧のみ取得
//   const problemsResp = await zabbixApiRequest('problem.get', {
//     output: ['eventid'],
//     suppressed: false
//   });
//   const problems = problemsResp.result || [];
//   const eventids = problems.map(p => p.eventid).filter(Boolean);
//   if (eventids.length === 0) {
//     return new Response(JSON.stringify([]), {
//       status: 200,
//       headers: { 'Content-Type': 'application/json' }
//     });
//   }

//   // event.getで詳細・hostsまとめて取得
//   const eventsResp = await zabbixApiRequest('event.get', {
//     output: ['eventid', 'clock', 'name'],
//     eventids: eventids,
//     selectHosts: ['hostid', 'host']
//   });
//   const events = (eventsResp.result || []).map(ev => {
//     // clock(UNIXタイム)→RFC3339(ISO8601)文字列へ変換
//     let clockIso = '';
//     if (ev.clock) {
//       const d = new Date(Number(ev.clock) * 1000);
//       clockIso = d.toISOString();
//     }
//     return {
//       eventid: ev.eventid,
//       clock: clockIso,
//       name: ev.name,
//       hosts: Array.isArray(ev.hosts) ? ev.hosts : []
//     };
//   });
//   return new Response(JSON.stringify(events), {
//     status: 200,
//     headers: { 'Content-Type': 'application/json' }
//   });
// }

// // Zabbix APIを呼び出す共通関数
// async function zabbixApiRequest(method, params) {
//   const payload = {
//     jsonrpc: '2.0',
//     method,
//     params,
//     id: 1
//   };
//   const apiResp = await fetch(ZABBIX_API_URL, {
//     method: 'POST',
//     headers: { 'Content-Type': 'application/json' },
//     body: JSON.stringify(payload),
//     credentials: 'same-origin'
//   });
//   return await apiResp.json();
// }
