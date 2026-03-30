const BASE_URL = 'http://[2400:4052:2962:5e00:5054:ff:fe8c:e2b2]/zabbix'

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
  const r = await fetch(`${BASE_URL}/api_jsonrpc.php`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer 7babc36130c77e3c4e20e3fd97e57d93c58c4c298311449449235106ca40840e'
    },
    body: JSON.stringify({ jsonrpc: '2.0', method, params, id: Date.now() })
  })
  const d = await r.json()
  if (d.error) throw new Error(`Zabbix API error: ${d.error.message} ${d.error.data}`)
  return d.result
}
