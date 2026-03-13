import React from 'react'
import { createRoot } from 'react-dom/client'
import { API_BASE } from './shared/api'
import './styles/login.css'

function LoginApp() {
  const [theme, setTheme] = React.useState(localStorage.getItem('zui.theme') || 'dark')
  const [username, setUsername] = React.useState(localStorage.getItem('rememberUser') || '')
  const [password, setPassword] = React.useState('')
  const [remember, setRemember] = React.useState(Boolean(localStorage.getItem('rememberUser')))
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState('')

  React.useEffect(() => {
    document.body.dataset.theme = theme
    localStorage.setItem('zui.theme', theme)
  }, [theme])

  async function handleSubmit(event) {
    event.preventDefault()
    if (!username.trim() || !password) {
      setError('请输入用户名和密码')
      return
    }
    setLoading(true)
    setError('')

    try {
      const response = await fetch(`${API_BASE}/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: username.trim(), password }),
      })
      const isJson = (response.headers.get('content-type') || '').includes('application/json')
      const data = isJson ? await response.json() : await response.text()
      if (!response.ok) {
        const message = (data && data.error) || (typeof data === 'string' ? data : '登录失败')
        throw new Error(message)
      }

      localStorage.setItem('token', data.token)
      if (remember) localStorage.setItem('rememberUser', username.trim())
      else localStorage.removeItem('rememberUser')
      window.location.href = 'main.html'
    } catch (e) {
      setError(e.message || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-shell">
      <form className="login-card" onSubmit={handleSubmit}>
        <div className="login-head">
          <h1>Z-UI</h1>
          <button className="theme-btn" type="button" onClick={() => setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'))}>
            {theme === 'dark' ? '☀︎' : '☾'}
          </button>
        </div>
        <p>欢迎回来，请登录管理面板</p>

        <label>用户名</label>
        <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="admin" />

        <label>密码</label>
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="••••••••" />

        <label className="remember">
          <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} />
          记住用户名
        </label>

        <button disabled={loading}>{loading ? '登录中...' : '登录'}</button>
        {error ? <div className="err">{error}</div> : null}
      </form>
    </div>
  )
}

createRoot(document.getElementById('root')).render(<LoginApp />)
