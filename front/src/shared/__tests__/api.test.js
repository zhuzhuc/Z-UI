import { describe, it, expect, vi, beforeEach } from 'vitest'
import { resolveApiBase } from '../api'

// Mock window.location
function mockLocation(hostname, port, origin, protocol = 'http:') {
  const loc = {
    hostname,
    port,
    origin: origin || `${protocol}//${hostname}${port ? ':' + port : ''}`,
    protocol,
  }
  vi.stubGlobal('location', loc)
}

describe('resolveApiBase', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('returns same-origin path for production (port 443)', () => {
    mockLocation('panel.example.com', '443', 'https://panel.example.com', 'https:')
    const result = resolveApiBase()
    expect(result).toBe('https://panel.example.com/api/v1')
  })

  it('returns same-origin for custom port (8080)', () => {
    mockLocation('myserver.com', '8080', 'http://myserver.com:8080')
    const result = resolveApiBase()
    expect(result).toBe('http://myserver.com:8080/api/v1')
  })

  it('returns fallback for dev server port 5500', () => {
    mockLocation('127.0.0.1', '5500', 'http://127.0.0.1:5500')
    const result = resolveApiBase()
    expect(result).toBe('http://127.0.0.1:8081/api/v1')
  })

  it('returns fallback for dev server port 5173', () => {
    mockLocation('localhost', '5173', 'http://localhost:5173')
    const result = resolveApiBase()
    expect(result).toBe('http://localhost:8081/api/v1')
  })

  it('returns fallback for localhost without port', () => {
    mockLocation('localhost', '', 'http://localhost')
    const result = resolveApiBase()
    expect(result).toBe('http://localhost:8081/api/v1')
  })

  it('returns fallback for 127.0.0.1 without port', () => {
    mockLocation('127.0.0.1', '', 'http://127.0.0.1')
    const result = resolveApiBase()
    expect(result).toBe('http://127.0.0.1:8081/api/v1')
  })

  it('returns same-origin for non-localhost with port 80', () => {
    mockLocation('example.com', '80', 'http://example.com')
    const result = resolveApiBase()
    expect(result).toBe('http://example.com/api/v1')
  })
})
