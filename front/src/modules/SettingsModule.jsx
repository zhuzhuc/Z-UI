import React from 'react'
import { api } from '../shared/api'

const initSettings = {
  title: 'Z-UI',
  language: 'zh-CN',
  theme: 'default',
  refreshIntervalSec: 5,
  requireLogin: true,
  allowRegister: false,
  enableTwoFactorLogin: false,
  publicBaseUrl: '',
}

export default function SettingsModule() {
  const [form, setForm] = React.useState(initSettings)
  const [username, setUsername] = React.useState('')
  const [oldPassword, setOldPassword] = React.useState('')
  const [newPassword, setNewPassword] = React.useState('')
  const [message, setMessage] = React.useState('')
  const [error, setError] = React.useState('')
  const [loading, setLoading] = React.useState(false)

  async function loadSettings() {
    setLoading(true)
    setError('')
    try {
      const res = await api('/panel/settings')
      setForm({
        title: res.title || 'Z-UI',
        language: res.language || 'zh-CN',
        theme: res.theme || 'default',
        refreshIntervalSec: Number(res.refreshIntervalSec || 5),
        requireLogin: !!res.requireLogin,
        allowRegister: !!res.allowRegister,
        enableTwoFactorLogin: !!res.enableTwoFactorLogin,
        publicBaseUrl: res.publicBaseUrl || '',
      })
      setUsername(res.adminUsername || '')
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  React.useEffect(() => {
    loadSettings()
  }, [])

  async function saveSettings(e) {
    e.preventDefault()
    setError('')
    try {
      await api('/panel/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      })
      setMessage('面板设置已保存')
    } catch (e2) {
      setError(e2.message)
    }
  }

  async function saveUsername(e) {
    e.preventDefault()
    setError('')
    try {
      const trimmed = String(username || '').trim()
      if (!trimmed) {
        setError('用户名不能为空')
        return
      }
      const res = await api('/auth/change-username', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: trimmed }),
      })
      const nextUsername = res.username || trimmed
      setUsername(nextUsername)
      localStorage.setItem('rememberUser', nextUsername)
      setMessage('用户名已更新，登录页已同步新用户名')
    } catch (e2) {
      setError(e2.message)
    }
  }

  async function savePassword(e) {
    e.preventDefault()
    setError('')
    try {
      if (!oldPassword || !newPassword) {
        setError('请输入旧密码和新密码')
        return
      }
      await api('/auth/change-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ oldPassword, newPassword }),
      })
      setOldPassword('')
      setNewPassword('')
      setMessage('密码已更新，下次登录请使用新密码')
    } catch (e2) {
      setError(e2.message)
    }
  }

  return (
    <div className="module-stack">
      <form className="panel" onSubmit={saveSettings}>
        <div className="head-row">
          <strong>面板设置</strong>
          <div className="toolbar">
            <button className="btn btn-ghost" type="button" onClick={loadSettings}>{loading ? '加载中...' : '刷新'}</button>
            <button className="btn btn-primary" type="submit">保存设置</button>
          </div>
        </div>

        <div className="grid two">
          <div className="field-block"><label>标题</label><input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} /></div>
          <div className="field-block"><label>语言</label><input value={form.language} onChange={(e) => setForm({ ...form, language: e.target.value })} /></div>
          <div className="field-block"><label>主题</label><input value={form.theme} onChange={(e) => setForm({ ...form, theme: e.target.value })} /></div>
          <div className="field-block"><label>刷新间隔（秒）</label><input type="number" min="1" value={form.refreshIntervalSec} onChange={(e) => setForm({ ...form, refreshIntervalSec: Number(e.target.value || 5) })} /></div>
          <div className="field-block"><label>公开地址</label><input value={form.publicBaseUrl} onChange={(e) => setForm({ ...form, publicBaseUrl: e.target.value })} /></div>
        </div>

        <div className="check-row">
          <label><input type="checkbox" checked={form.requireLogin} onChange={(e) => setForm({ ...form, requireLogin: e.target.checked })} /> 需要登录</label>
          <label><input type="checkbox" checked={form.allowRegister} onChange={(e) => setForm({ ...form, allowRegister: e.target.checked })} /> 允许注册</label>
          <label><input type="checkbox" checked={form.enableTwoFactorLogin} onChange={(e) => setForm({ ...form, enableTwoFactorLogin: e.target.checked })} /> 启用 2FA</label>
        </div>
      </form>

      <form className="panel" onSubmit={saveUsername}>
        <div className="head-row"><strong>账号设置</strong></div>
        <div className="inline-form">
          <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="新用户名" />
          <button className="btn btn-primary" type="submit">修改用户名</button>
        </div>
      </form>

      <form className="panel" onSubmit={savePassword}>
        <div className="head-row"><strong>密码设置</strong></div>
        <div className="inline-form">
          <input type="password" value={oldPassword} onChange={(e) => setOldPassword(e.target.value)} placeholder="旧密码" />
          <input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder="新密码" />
          <button className="btn btn-primary" type="submit">修改密码</button>
        </div>
      </form>

      <div className={`hint ${error ? 'err' : ''}`}>{error || message}</div>
    </div>
  )
}
