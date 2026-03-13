;(function () {
  function resolveApiBase() {
    var fallback = 'http://127.0.0.1:8081/api/v1'
    if (!window.location || !window.location.origin || !window.location.origin.startsWith('http')) {
      return fallback
    }

    var hostname = String(window.location.hostname || '').toLowerCase()
    var port = String(window.location.port || '')
    var forceBackendPorts = new Set(['5500', '5173', '4173', '3000'])
    var isLocalHost = hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '::1' || hostname === '[::1]' || hostname === '0.0.0.0'

    if (forceBackendPorts.has(port)) return fallback
    if (isLocalHost && (port === '' || port === '80' || port === '443')) return fallback
    return window.location.origin + '/api/v1'
  }

  window.ZUIShared = {
    resolveApiBase: resolveApiBase
  }
})()

