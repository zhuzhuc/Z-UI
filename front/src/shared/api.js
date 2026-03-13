export function resolveApiBase() {
  const fallback = 'http://127.0.0.1:8081/api/v1'
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

export async function api(path, options = {}) {
  const token = localStorage.getItem('token')
  const headers = {
    ...(options.headers || {}),
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  }

  let response
  try {
    response = await fetch(`${API_BASE}${path}`, { ...options, headers })
  } catch {
    throw new Error('网络异常，请检查服务和网络连接')
  }

  const isJson = (response.headers.get('content-type') || '').includes('application/json')
  const data = isJson ? await response.json() : await response.text()

  if (response.status === 401) {
    localStorage.removeItem('token')
    window.location.href = 'login.html'
    throw new Error('登录已过期，请重新登录')
  }

  if (!response.ok) {
    const message = (data && data.error) || (typeof data === 'string' ? data : '请求失败')
    throw new Error(message)
  }

  return data
}

