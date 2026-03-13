import React from 'react'
import { api } from '../shared/api'

export default function LogsModule() {
  const [type, setType] = React.useState('xray')
  const [xrayTarget, setXrayTarget] = React.useState('auto')
  const [lines, setLines] = React.useState(300)
  const [auto, setAuto] = React.useState(true)
  const [content, setContent] = React.useState('')
  const [auditItems, setAuditItems] = React.useState([])
  const [auditQuery, setAuditQuery] = React.useState('')
  const [auditAction, setAuditAction] = React.useState('all')
  const [source, setSource] = React.useState('-')
  const [updatedAt, setUpdatedAt] = React.useState('')
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState('')

  const fetchLogs = React.useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      if (type === 'xray') {
        const res = await api(`/logs/xray?target=${encodeURIComponent(xrayTarget)}&lines=${Number(lines) || 200}`)
        setContent(res.content || '')
        setAuditItems([])
        setSource(res.source || '-')
        setUpdatedAt(res.updatedAt || '')
      } else if (type === 'system') {
        const res = await api(`/logs/system?lines=${Number(lines) || 200}`)
        setContent(res.content || '')
        setAuditItems([])
        setSource(res.source || '-')
        setUpdatedAt(res.updatedAt || '')
      } else {
        const limit = Math.min(Math.max(Number(lines) || 100, 1), 1000)
        const res = await api(`/audit/logs?limit=${limit}&offset=0`)
        setAuditItems(res.items || [])
        setContent('')
        setSource('audit_logs')
        setUpdatedAt(new Date().toISOString())
      }
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [type, xrayTarget, lines])

  React.useEffect(() => {
    fetchLogs()
  }, [fetchLogs])

  React.useEffect(() => {
    if (!auto) return undefined
    const timer = window.setInterval(fetchLogs, 3000)
    return () => window.clearInterval(timer)
  }, [auto, fetchLogs])

  const shownAudit = auditItems.filter((item) => {
    const q = auditQuery.trim().toLowerCase()
    const byQuery = !q || [item.action, item.target, item.detail, item.username, item.ip].join(' ').toLowerCase().includes(q)
    const byAction = auditAction === 'all' || String(item.action || '').startsWith(auditAction)
    return byQuery && byAction
  })

  const auditActions = React.useMemo(() => {
    const set = new Set()
    auditItems.forEach((item) => {
      const action = String(item.action || '').trim()
      if (action) set.add(action.split('.')[0])
    })
    return ['all', ...Array.from(set).sort()]
  }, [auditItems])

  function exportAuditCSV() {
    if (shownAudit.length === 0) return
    const header = ['time', 'username', 'ip', 'action', 'target', 'detail']
    const escapeCell = (val) => `"${String(val ?? '').replace(/"/g, '""')}"`
    const rows = shownAudit.map((item) => [
      item.createdAt ? new Date(item.createdAt).toISOString() : '',
      item.username || '',
      item.ip || '',
      item.action || '',
      item.target || '',
      item.detail || '',
    ])
    const csv = [header, ...rows].map((row) => row.map(escapeCell).join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `audit-logs-${Date.now()}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="module-stack">
      <div className="panel">
        <div className="head-row">
          <strong>日志中心</strong>
          <div className="toolbar">
            <button className="btn btn-ghost" onClick={fetchLogs}>{loading ? '刷新中...' : '刷新'}</button>
          </div>
        </div>

        <div className="toolbar">
          <select value={type} onChange={(e) => setType(e.target.value)}>
            <option value="xray">Xray 日志</option>
            <option value="system">系统日志</option>
            <option value="audit">审计日志</option>
          </select>
          {type === 'xray' ? (
            <select value={xrayTarget} onChange={(e) => setXrayTarget(e.target.value)}>
              <option value="auto">自动</option>
              <option value="error">error</option>
              <option value="access">access</option>
              <option value="journal">journalctl</option>
            </select>
          ) : null}
          {type === 'audit' ? (
            <input value={auditQuery} onChange={(e) => setAuditQuery(e.target.value)} placeholder="搜索 action/user/ip/detail" />
          ) : null}
          {type === 'audit' ? (
            <select value={auditAction} onChange={(e) => setAuditAction(e.target.value)}>
              {auditActions.map((action) => (
                <option key={action} value={action}>{action === 'all' ? '全部动作' : action}</option>
              ))}
            </select>
          ) : null}
          <input type="number" min={type === 'audit' ? '1' : '50'} max="5000" value={lines} onChange={(e) => setLines(Number(e.target.value || 200))} />
          <label className="check-line"><input type="checkbox" checked={auto} onChange={(e) => setAuto(e.target.checked)} /> 自动刷新</label>
          {type === 'audit' ? (
            <button className="btn btn-ghost" onClick={exportAuditCSV}>导出 CSV</button>
          ) : null}
        </div>

        <div className="meta-line">
          <span>来源：{source}</span>
          <span>更新时间：{updatedAt ? new Date(updatedAt).toLocaleString() : '-'}</span>
        </div>

        {type === 'audit' ? (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>时间</th>
                  <th>用户</th>
                  <th>IP</th>
                  <th>动作</th>
                  <th>目标</th>
                  <th>详情</th>
                </tr>
              </thead>
              <tbody>
                {shownAudit.map((item) => (
                  <tr key={item.id}>
                    <td>{item.createdAt ? new Date(item.createdAt).toLocaleString() : '-'}</td>
                    <td>{item.username || '-'}</td>
                    <td>{item.ip || '-'}</td>
                    <td>{item.action || '-'}</td>
                    <td>{item.target || '-'}</td>
                    <td>{item.detail || '-'}</td>
                  </tr>
                ))}
                {shownAudit.length === 0 ? (
                  <tr><td colSpan="6">暂无审计日志</td></tr>
                ) : null}
              </tbody>
            </table>
          </div>
        ) : (
          <pre className="log-pre">{content || (error ? '' : '暂无日志')}</pre>
        )}
      </div>

      <div className={`hint ${error ? 'err' : ''}`}>{error || '日志读取正常'}</div>
    </div>
  )
}
