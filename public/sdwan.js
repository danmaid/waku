const BASE_URL = '/dataservice'
const auth = { j_username: 'cisco', j_password: 'cisco' }

export async function getDevices() {
  await fetch(`${BASE_URL}/j_security_check`)
  const r = await fetch(`${BASE_URL}/j_security_check`, {
    method: 'POST',
    body: new URLSearchParams(auth)
  })
  if (r.ok) {
    const r = await fetch(`${BASE_URL}/device`)
    const devices = await r.json()
    console.log('Devices:', devices)
    return devices.data
  } else {
    console.error('Failed to login:', r.status, r.statusText)
    throw r
  }
}