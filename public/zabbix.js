/** @type {() => Promise<{eventid: string}[]>} */
export async function getProblems() {
  const d = await callZabbixApi('problem.get', { output: 'extend' })
  if (!Array.isArray(d)) throw new Error('Unexpected Zabbix API response for problem.get')
  return d
}

/** @type {(options?: { eventids?: string[] }) => Promise<{eventid: string}[]>} */
export async function getEvents(options = {}) {
  const d = await callZabbixApi('event.get', { ...options, output: ['eventid'] })
  if (!Array.isArray(d)) throw new Error('Unexpected Zabbix API response for event.get')
  return d
}

/** @type {() => Promise<{hostid: string, host: string, name: string}[]>} */
export async function getHosts() {
  const d = await callZabbixApi('host.get', {
    output: ['hostid', 'host', 'name']
  })
  if (!Array.isArray(d)) throw new Error('Unexpected Zabbix API response for host.get')
  return d
}

/** @type {(method: string, params?: object) => Promise<any>} */
async function callZabbixApi(method, params = {}) {
  const r = await fetch('/zabbix/api_jsonrpc.php', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', method, params, id: Date.now() })
  })
  const d = await r.json();
  if (d.error) throw new Error(`Zabbix API error: ${d.error.message}`)
  return d.result
}
