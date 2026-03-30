const BASE_URL = 'https://[2400:4052:2962:5e00::500:1]/dna'
console.log(BASE_URL)
const auth = `Basic ${btoa('cisco:ZAQ!2wsx')}`
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
} else {
  console.error('Failed to get token:', d)
}
