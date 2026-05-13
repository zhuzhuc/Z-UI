export function resolveApiBase() {
  const fallback = `http://${window.location?.hostname || '127.0.0.1'}:8081/api/v1`
  if (!window.location || !window.location.origin || !window.location.origin.startsWith('http')) {
    return fallback
  }

  const hostname = String(window.location.hostname || '').toLowerCase()
  const port = String(window.location.port || '')
  const forceBackendPorts = new Set(['5500', '5173', '4173', '3000'])
  const isLocalHost = ['localhost', '127.0.0.1', '::1', '[::1]', '0.0.0.0'].includes(hostname)

  if (forceBackendPorts.has(port)) return fallback
  if (isLocalHost && (port === '' || port === '80' || port === '443')) return fallback
  return `${window.location.origin}/api/v1`
}

export const API_BASE = resolveApiBase()

let _redirecting = false

export async function api(path, options = {}) {
  const headers = {
    ...(options.headers || {}),
  }

  const controller = new AbortController()
  const timeoutMs = options.timeout || 15000
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs)

  let response
  try {
    response = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers,
      credentials: 'include',
      signal: controller.signal,
    })
  } catch (err) {
    clearTimeout(timeoutId)
    if (err.name === 'AbortError') {
      throw new Error('request timeout')
    }
    throw new Error('network error, please check your connection')
  }
  clearTimeout(timeoutId)

  const isJson = (response.headers.get('content-type') || '').includes('application/json')
  const data = isJson ? await response.json() : await response.text()

  if (response.status === 401) {
    if (!_redirecting) {
      _redirecting = true
      window.location.href = 'login.html'
    }
    throw new Error('session expired, please login again')
  }

  if (!response.ok) {
    const message = (data && data.error) || (typeof data === 'string' ? data : 'request failed')
    throw new Error(message)
  }

  return data
}
