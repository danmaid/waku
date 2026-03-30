const BASE_URL = '/dna'
const auth = `Basic Y2lzY286WkFRITJ3c3g=`

export async function getDevices() {
  const r = await fetch(`${BASE_URL}/system/api/v1/auth/token`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': auth
    }
  })
  const d = await r.json()
  if (d.Token) {
    console.log('Token:', d.Token)
    const r = await fetch(`${BASE_URL}/intent/api/v1/network-device`, {
      headers: { 'X-Auth-Token': d.Token }
    })
    const devices = await r.json()
    console.log('Devices:', devices)
    return devices.response
  } else {
    console.error('Failed to get token:', d)
  }
  throw Error('Failed to get token')
}