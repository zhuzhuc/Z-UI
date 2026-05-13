import React from 'react'
import { createRoot } from 'react-dom/client'
import { API_BASE } from './shared/api'
import { useT } from './shared/i18n'
import './styles/login.css'

function LoginApp() {
  const savedLang = localStorage.getItem('zui.lang') || 'zh'
  const savedTheme = localStorage.getItem('zui.theme') || ''
  const systemPrefersDark = typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches

  const [lang, setLang] = React.useState(savedLang)
  const [theme, setTheme] = React.useState(savedTheme || (systemPrefersDark ? 'dark' : 'light'))
  const t = useT(lang)

  const [username, setUsername] = React.useState(localStorage.getItem('rememberUser') || '')
  const [password, setPassword] = React.useState('')
  const [remember, setRemember] = React.useState(Boolean(localStorage.getItem('rememberUser')))
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState('')

  React.useEffect(() => {
    document.body.dataset.theme = theme
    localStorage.setItem('zui.theme', theme)
  }, [theme])

  React.useEffect(() => {
    localStorage.setItem('zui.lang', lang)
  }, [lang])

  async function handleSubmit(event) {
    event.preventDefault()
    if (!username.trim() || !password) {
      setError(t('enterUsernamePassword'))
      return
    }
    setLoading(true)
    setError('')

    try {
      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), 15000)
      const response = await fetch(`${API_BASE}/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: username.trim(), password }),
        signal: controller.signal,
      })
      clearTimeout(timeoutId)
      const isJson = (response.headers.get('content-type') || '').includes('application/json')
      const data = isJson ? await response.json() : await response.text()
      if (!response.ok) {
        const message = (data && data.error) || (typeof data === 'string' ? data : t('loginFailed'))
        throw new Error(message)
      }

      if (remember) localStorage.setItem('rememberUser', username.trim())
      else localStorage.removeItem('rememberUser')
      window.location.href = 'main.html'
    } catch (e) {
      if (e.name === 'AbortError') {
        setError(t('networkError'))
      } else {
        setError(e.message || t('loginFailed'))
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-shell">
      <form className="login-card" onSubmit={handleSubmit}>
        <h1>Z-PANEL</h1>
        <p>{t('loginTitle')}</p>

        <label htmlFor="login-username">{t('username')}</label>
        <input
          id="login-username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="admin"
          autoComplete="username"
          maxLength={64}
        />

        <label htmlFor="login-password">{t('password')}</label>
        <input
          id="login-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="••••••••"
          autoComplete="current-password"
        />

        <label className="remember">
          <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} />
          {t('rememberUsername')}
        </label>

        <button disabled={loading}>{loading ? t('loginLoading') : t('loginButton')}</button>
        {error ? <div className="err" role="alert">{error}</div> : null}

        <div className="login-extras">
          <button type="button" className="link-btn" onClick={() => setLang((p) => (p === 'zh' ? 'en' : 'zh'))}>
            {lang === 'zh' ? 'English' : '中文'}
          </button>
          <button type="button" className="link-btn" onClick={() => setTheme((p) => (p === 'dark' ? 'light' : 'dark'))}>
            {theme === 'dark' ? '☀️ Light' : '🌙 Dark'}
          </button>
        </div>
      </form>
    </div>
  )
}

createRoot(document.getElementById('root')).render(<LoginApp />)
