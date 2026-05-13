import React from 'react'
import { api } from '../shared/api'
import ConfirmDialog from '../components/ConfirmDialog'

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

const roleOptions = [
  { value: 'owner', label: 'Owner' },
  { value: 'admin', label: 'Admin' },
  { value: 'operator', label: 'Operator' },
  { value: 'viewer', label: 'Viewer' },
]

export default function SettingsModule({
  onSettingsChange = () => {},
  currentUser = null,
  currentUserRole = 'viewer',
  onUsernameChange = () => {},
  onProfileRefresh = () => {},
  t = (k, f) => f || k,
  lang = 'zh',
}) {
  const [form, setForm] = React.useState(initSettings)
  const [username, setUsername] = React.useState('')
  const [oldPassword, setOldPassword] = React.useState('')
  const [newPassword, setNewPassword] = React.useState('')
  const [message, setMessage] = React.useState('')
  const [error, setError] = React.useState('')
  const [loading, setLoading] = React.useState(false)
  const [users, setUsers] = React.useState([])
  const [usersLoading, setUsersLoading] = React.useState(false)
  const [newUser, setNewUser] = React.useState({ username: '', password: '', role: 'viewer' })
  const [confirm, setConfirm] = React.useState(null)
  const [backupLoading, setBackupLoading] = React.useState(false)
  const [restoreLoading, setRestoreLoading] = React.useState(false)
  const restoreInputRef = React.useRef(null)

  const zh = lang !== 'en'
  const canManageUsers = currentUserRole === 'owner' || currentUserRole === 'admin'

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
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  React.useEffect(() => {
    loadSettings()
  }, [])

  React.useEffect(() => {
    setUsername(currentUser?.username || '')
  }, [currentUser?.username])

  React.useEffect(() => {
    if (!canManageUsers) {
      setUsers([])
      return
    }
    loadUsers()
  }, [canManageUsers])

  async function saveSettings(e) {
    e.preventDefault()
    setError('')
    try {
      const res = await api('/panel/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      })
      onSettingsChange(res)
      setMessage(zh ? '面板设置已保存' : 'Settings saved')
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
        setError(zh ? '用户名不能为空' : 'Username cannot be empty')
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
      setMessage(zh ? '用户名已更新' : 'Username updated')
    } catch (e2) {
      setError(e2.message)
    }
  }

  async function savePassword(e) {
    e.preventDefault()
    setError('')
    try {
      if (!oldPassword || !newPassword) {
        setError(zh ? '请输入旧密码和新密码' : 'Please enter old and new password')
        return
      }
      await api('/auth/change-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ oldPassword, newPassword }),
      })
      setOldPassword('')
      setNewPassword('')
      onProfileRefresh()
      setMessage(zh ? '密码已更新' : 'Password updated')
    } catch (e2) {
      setError(e2.message)
    }
  }

  async function loadUsers() {
    if (!canManageUsers) return
    setUsersLoading(true)
    setError('')
    try {
      const res = await api('/users')
      setUsers(res.items || [])
    } catch (e) {
      setError(e.message)
    } finally {
      setUsersLoading(false)
    }
  }

  async function createUser(e) {
    e.preventDefault()
    setError('')
    const trimmed = String(newUser.username || '').trim()
    if (!trimmed || !newUser.password) {
      setError(zh ? '请输入用户名和密码' : 'Please enter username and password')
      return
    }
    try {
      await api('/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: trimmed, password: newUser.password, role: newUser.role }),
      })
      setMessage(zh ? '用户已创建' : 'User created')
      setNewUser({ username: '', password: '', role: newUser.role })
      loadUsers()
    } catch (e2) {
      setError(e2.message)
    }
  }

  async function changeUserRole(id, role) {
    setError('')
    try {
      await api(`/users/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ role }),
      })
      setMessage(zh ? '用户角色已更新' : 'User role updated')
      loadUsers()
      if (currentUser?.id === id) onProfileRefresh()
    } catch (e2) {
      setError(e2.message)
    }
  }

  function resetUserPassword(id) {
    const pwd = window.prompt(zh ? '输入新密码（至少10位，需包含大小写字母和数字）' : 'Enter new password (min 10 chars, upper/lower/number)')
    if (!pwd) return
    api(`/users/${id}/password`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: pwd }),
    }).then(() => {
      setMessage(zh ? '已重置该用户密码' : 'Password reset')
      if (currentUser?.id === id) onProfileRefresh()
    }).catch((e) => setError(e.message))
  }

  function deleteUser(id, uname) {
    setConfirm({
      title: zh ? '删除用户' : 'Delete User',
      message: zh ? `确认删除用户 "${uname}" ?` : `Delete user "${uname}"?`,
      onConfirm: async () => {
        setConfirm(null)
        try {
          await api(`/users/${id}`, { method: 'DELETE' })
          setMessage(zh ? '用户已删除' : 'User deleted')
          loadUsers()
        } catch (e) {
          setError(e.message)
        }
      },
    })
  }

  async function downloadBackup() {
    setBackupLoading(true)
    try {
      const res = await fetch('/api/v1/backup/download', { credentials: 'include' })
      if (!res.ok) throw new Error(zh ? '备份下载失败' : 'Backup download failed')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `z-ui-backup-${new Date().toISOString().slice(0, 10)}.zip`
      a.click()
      URL.revokeObjectURL(url)
      setMessage(zh ? '备份已下载' : 'Backup downloaded')
    } catch (e) {
      setError(e.message)
    } finally {
      setBackupLoading(false)
    }
  }

  function confirmRestore(file) {
    setConfirm({
      title: zh ? '恢复备份' : 'Restore Backup',
      message: zh ? '恢复将覆盖当前数据，确认继续?' : 'Restore will overwrite current data. Continue?',
      onConfirm: async () => {
        setConfirm(null)
        setRestoreLoading(true)
        try {
          const fd = new FormData()
          fd.append('file', file)
          const res = await fetch('/api/v1/backup/restore', {
            method: 'POST',
            credentials: 'include',
            body: fd,
          })
          if (!res.ok) {
            const data = await res.json().catch(() => ({}))
            throw new Error(data.error || (zh ? '恢复失败' : 'Restore failed'))
          }
          setMessage(zh ? '恢复成功，请刷新页面' : 'Restore successful, please refresh')
        } catch (e) {
          setError(e.message)
        } finally {
          setRestoreLoading(false)
        }
      },
    })
  }

  return (
    <div className="module-stack">
      <form className="panel" onSubmit={saveSettings}>
        <div className="head-row">
          <strong>{zh ? '面板设置' : 'Panel Settings'}</strong>
          <div className="toolbar">
            <button className="btn btn-ghost" type="button" onClick={loadSettings}>{loading ? (zh ? '加载中...' : 'Loading...') : (zh ? '刷新' : 'Refresh')}</button>
            <button className="btn btn-primary" type="submit">{zh ? '保存设置' : 'Save Settings'}</button>
          </div>
        </div>

        <div className="grid two">
          <div className="field-block"><label>{zh ? '标题' : 'Title'}</label><input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} /></div>
          <div className="field-block"><label>{zh ? '语言' : 'Language'}</label><input value={form.language} onChange={(e) => setForm({ ...form, language: e.target.value })} /></div>
          <div className="field-block"><label>{zh ? '主题' : 'Theme'}</label><input value={form.theme} onChange={(e) => setForm({ ...form, theme: e.target.value })} /></div>
          <div className="field-block"><label>{zh ? '刷新间隔(秒)' : 'Refresh Interval (sec)'}</label><input type="number" min="1" value={form.refreshIntervalSec} onChange={(e) => setForm({ ...form, refreshIntervalSec: Number(e.target.value || 5) })} /></div>
          <div className="field-block"><label>{zh ? '公共访问地址' : 'Public Base URL'}</label><input value={form.publicBaseUrl} onChange={(e) => setForm({ ...form, publicBaseUrl: e.target.value })} /></div>
        </div>

        <div className="check-row">
          <label><input type="checkbox" checked={form.requireLogin} onChange={(e) => setForm({ ...form, requireLogin: e.target.checked })} /> {zh ? '需要登录' : 'Require Login'}</label>
          <label><input type="checkbox" checked={form.allowRegister} onChange={(e) => setForm({ ...form, allowRegister: e.target.checked })} /> {zh ? '允许注册' : 'Allow Register'}</label>
          <label><input type="checkbox" checked={form.enableTwoFactorLogin} onChange={(e) => setForm({ ...form, enableTwoFactorLogin: e.target.checked })} /> {zh ? '启用 2FA' : 'Enable 2FA'}</label>
        </div>
      </form>

      <form className="panel" onSubmit={saveUsername}>
        <div className="head-row"><strong>{zh ? '账号设置' : 'Account Settings'}</strong></div>
        <div className="inline-form">
          <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder={zh ? '新用户名' : 'New username'} />
          <button className="btn btn-primary" type="submit">{zh ? '修改用户名' : 'Change Username'}</button>
        </div>
      </form>

      <form className="panel" onSubmit={savePassword}>
        <div className="head-row"><strong>{zh ? '密码设置' : 'Password Settings'}</strong></div>
        <div className="inline-form">
          <input type="password" value={oldPassword} onChange={(e) => setOldPassword(e.target.value)} placeholder={zh ? '当前密码' : 'Current password'} />
          <input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder={zh ? '新密码' : 'New password'} />
          <button className="btn btn-primary" type="submit">{zh ? '修改密码' : 'Change Password'}</button>
        </div>
      </form>

      {canManageUsers ? (
        <div className="panel">
          <div className="head-row">
            <strong>{zh ? '用户管理' : 'User Management'}</strong>
            <div className="toolbar">
              <button className="btn btn-ghost" type="button" onClick={loadUsers}>{usersLoading ? (zh ? '刷新中...' : 'Loading...') : (zh ? '刷新' : 'Refresh')}</button>
            </div>
          </div>

          <form className="grid two" onSubmit={createUser}>
            <div className="field-block">
              <label>{zh ? '用户名' : 'Username'}</label>
              <input value={newUser.username} onChange={(e) => setNewUser({ ...newUser, username: e.target.value })} placeholder="new-user" />
            </div>
            <div className="field-block">
              <label>{zh ? '密码' : 'Password'}</label>
              <input type="password" value={newUser.password} onChange={(e) => setNewUser({ ...newUser, password: e.target.value })} placeholder={zh ? '至少10位，含大小写和数字' : 'Min 10 chars'} />
            </div>
            <div className="field-block">
              <label>{zh ? '角色' : 'Role'}</label>
              <select value={newUser.role} onChange={(e) => setNewUser({ ...newUser, role: e.target.value })}>
                {roleOptions.map((opt) => (
                  <option key={opt.value} value={opt.value}>{opt.label}</option>
                ))}
              </select>
            </div>
            <div className="field-block" style={{ display: 'flex', alignItems: 'flex-end' }}>
              <button className="btn btn-primary" type="submit">{zh ? '创建用户' : 'Create User'}</button>
            </div>
          </form>

          <div className="table-wrap" style={{ marginTop: 20 }}>
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>{zh ? '用户名' : 'Username'}</th>
                  <th>{zh ? '角色' : 'Role'}</th>
                  <th>{zh ? '状态' : 'Status'}</th>
                  <th>{zh ? '创建时间' : 'Created'}</th>
                  <th>{zh ? '操作' : 'Actions'}</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => {
                  const isSelf = currentUser?.id === user.id
                  const isOwner = user.role === 'owner'
                  const disableOwnerAction = isOwner && currentUserRole !== 'owner'
                  return (
                    <tr key={user.id}>
                      <td>{user.id}</td>
                      <td>{user.username}</td>
                      <td>
                        <select
                          value={user.role}
                          disabled={isSelf || disableOwnerAction}
                          onChange={(e) => changeUserRole(user.id, e.target.value)}
                        >
                          {roleOptions.map((opt) => (
                            <option key={opt.value} value={opt.value}>{opt.label}</option>
                          ))}
                        </select>
                      </td>
                      <td>{user.status || 'active'}</td>
                      <td>{user.createdAt ? new Date(user.createdAt).toLocaleString() : '-'}</td>
                      <td>
                        <button className="btn btn-ghost" type="button" disabled={disableOwnerAction} onClick={() => resetUserPassword(user.id)}>
                          {zh ? '重置密码' : 'Reset Password'}
                        </button>
                        <button className="btn btn-danger" type="button" disabled={isSelf || disableOwnerAction} onClick={() => deleteUser(user.id, user.username)}>
                          {zh ? '删除' : 'Delete'}
                        </button>
                      </td>
                    </tr>
                  )
                })}
                {users.length === 0 ? (
                  <tr><td colSpan="6">{zh ? '暂无用户' : 'No users'}</td></tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </div>
      ) : null}

      {canManageUsers ? (
        <div className="panel">
          <div className="head-row">
            <strong>{zh ? '备份与恢复' : 'Backup & Restore'}</strong>
          </div>
          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
            <button className="btn btn-primary" onClick={downloadBackup} disabled={backupLoading}>
              {backupLoading ? (zh ? '下载中...' : 'Downloading...') : (zh ? '下载备份' : 'Download Backup')}
            </button>
            <button className="btn btn-ghost" onClick={() => restoreInputRef.current?.click()} disabled={restoreLoading}>
              {restoreLoading ? (zh ? '恢复中...' : 'Restoring...') : (zh ? '上传恢复' : 'Upload Restore')}
            </button>
          </div>
          <input
            ref={restoreInputRef}
            type="file"
            accept=".zip"
            style={{ display: 'none' }}
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) confirmRestore(file)
              e.target.value = ''
            }}
          />
        </div>
      ) : null}

      <ConfirmDialog open={!!confirm} title={confirm?.title} message={confirm?.message} onConfirm={confirm?.onConfirm} onCancel={() => setConfirm(null)} />
      <div className={`hint ${error ? 'err' : ''}`}>{error || message}</div>
    </div>
  )
}
