import React from 'react'
import { api } from '../shared/api'
import ConfirmDialog from '../components/ConfirmDialog'

export default function XrayConfigModule({ t = (k, f) => f || k, lang = 'zh' }) {
  const [config, setConfig] = React.useState('')
  const [editing, setEditing] = React.useState(false)
  const [draft, setDraft] = React.useState('')
  const [loading, setLoading] = React.useState(false)
  const [message, setMessage] = React.useState('')
  const [error, setError] = React.useState('')
  const [confirm, setConfirm] = React.useState(null)
  const [xrayStatus, setXrayStatus] = React.useState(null)

  const zh = lang !== 'en'

  async function loadConfig() {
    setLoading(true)
    setError('')
    try {
      const [cfg, status] = await Promise.all([
        api('/xray/config'),
        api('/xray/status').catch(() => null),
      ])
      const text = typeof cfg === 'string' ? cfg : JSON.stringify(cfg, null, 2)
      setConfig(text)
      setDraft(text)
      setXrayStatus(status)
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  React.useEffect(() => {
    loadConfig()
  }, [])

  function startEdit() {
    setDraft(config)
    setEditing(true)
  }

  function cancelEdit() {
    setDraft(config)
    setEditing(false)
  }

  function applyConfig() {
    setConfirm({
      title: zh ? '应用配置' : 'Apply Config',
      message: zh ? '确认应用新配置？Xray 将重启。' : 'Apply new config? Xray will restart.',
      onConfirm: async () => {
        setConfirm(null)
        setError('')
        try {
          let parsed
          try {
            parsed = JSON.parse(draft)
          } catch {
            setError(zh ? 'JSON 格式错误' : 'Invalid JSON')
            return
          }
          await api('/xray/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(parsed),
          })
          setEditing(false)
          setMessage(zh ? '配置已应用' : 'Config applied')
          await loadConfig()
        } catch (e) {
          setError(e.message)
        }
      },
    })
  }

  async function restartXray() {
    setError('')
    try {
      await api('/xray/restart', { method: 'POST' })
      setMessage(zh ? 'Xray 已重启' : 'Xray restarted')
      const status = await api('/xray/status').catch(() => null)
      setXrayStatus(status)
    } catch (e) {
      setError(e.message)
    }
  }

  return (
    <div className="panel">
      <div className="head-row">
        <strong>{zh ? 'Xray 配置' : 'Xray Config'}</strong>
        <div className="toolbar">
          <button className="btn btn-ghost" onClick={loadConfig}>{loading ? (zh ? '加载中...' : 'Loading...') : (zh ? '刷新' : 'Refresh')}</button>
          {!editing ? (
            <button className="btn btn-ghost" onClick={startEdit}>{zh ? '编辑配置' : 'Edit Config'}</button>
          ) : (
            <>
              <button className="btn btn-primary" onClick={applyConfig}>{zh ? '应用配置' : 'Apply Config'}</button>
              <button className="btn btn-ghost" onClick={cancelEdit}>{zh ? '取消' : 'Cancel'}</button>
            </>
          )}
          <button className="btn btn-ghost" onClick={restartXray}>{zh ? '重启 Xray' : 'Restart Xray'}</button>
        </div>
      </div>

      {xrayStatus ? (
        <div style={{ marginBottom: 12, display: 'flex', gap: 16, fontSize: 13 }}>
          <span>{zh ? '状态' : 'Status'}: <strong>{xrayStatus.running ? (zh ? '运行中' : 'Running') : (zh ? '已停止' : 'Stopped')}</strong></span>
          {xrayStatus.pid ? <span>PID: {xrayStatus.pid}</span> : null}
          {xrayStatus.version ? <span>Version: {xrayStatus.version}</span> : null}
        </div>
      ) : null}

      <textarea
        value={editing ? draft : config}
        onChange={(e) => setDraft(e.target.value)}
        readOnly={!editing}
        style={{
          width: '100%',
          minHeight: 400,
          fontFamily: 'monospace',
          fontSize: 13,
          lineHeight: 1.5,
          padding: 12,
          border: '1px solid var(--border)',
          borderRadius: 6,
          background: editing ? 'var(--bg)' : 'var(--surface)',
          color: 'var(--text)',
          resize: 'vertical',
          tabSize: 2,
        }}
        spellCheck={false}
      />

      <ConfirmDialog open={!!confirm} title={confirm?.title} message={confirm?.message} onConfirm={confirm?.onConfirm} onCancel={() => setConfirm(null)} />
      <div className={`hint ${error ? 'err' : ''}`}>{error || message}</div>
    </div>
  )
}
