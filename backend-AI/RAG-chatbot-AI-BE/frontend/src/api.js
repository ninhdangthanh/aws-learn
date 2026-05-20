const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

export async function uploadDocument(file) {
  const fd = new FormData()
  fd.append('file', file)
  const res = await fetch(`${API_BASE}/api/v1/documents`, {method: 'POST', body: fd})
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function chatRequest(body) {
  const res = await fetch(`${API_BASE}/api/v1/chat`, {method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify(body)})
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
