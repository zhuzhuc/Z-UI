import React from 'react'
import { api } from '../shared/api'

function fmtBytes(bytes) {
  const value = Number(bytes || 0)
  if (value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const idx = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  return `${(value / 1024 ** idx).toFixed(idx === 0 ? 0 : 2)} ${units[idx]}`
}

export default function SubscriptionModule() {
  const [info, setInfo] = React.useState(null)
  const [preview, setPreview] = React.useState(null)
  const [nodes, setNodes] = React.useState([])
  const [statsMap, setStatsMap] = React.useState({})
  const [loading, setLoading] = React.useState(false)
  const [message, setMessage] = React.useState('')
  const [error, setError] = React.useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const [infoRes, previewRes, nodesRes] = await Promise.all([
        api('/subscription/info'),
        api('/subscription/preview'),
        api('/subscription/nodes'),
      ])
      setInfo(infoRes)
      setPreview(previewRes)
      setNodes(nodesRes.items || [])
      setMessage(`共 ${(nodesRes.items || []).length} 个入站，${nodesRes.linkCount || 0} 条节点链接`)

      try {
        const overview = await api('/xray/stats/overview')
        const mapped = {}
        ;(overview.userTraffic || []).forEach((item) => {
          mapped[String(item.user || '').trim()] = item
        })
        setStatsMap(mapped)
      } catch {
        setStatsMap({})
      }
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  React.useEffect(() => {
    load()
  }, [])

  async function rotateToken() {
    try {
      await api('/subscription/rotate', { method: 'POST' })
      setMessage('订阅令牌已重置')
      await load()
    } catch (e) {
      setError(e.message)
    }
  }

  async function copyText(text) {
    try {
      await navigator.clipboard.writeText(text)
      setMessage('已复制到剪贴板')
    } catch {
      setError('复制失败，请手动复制')
    }
  }

  return (
    <div className="module-stack">
      <div className="panel">
        <div className="head-row">
          <strong>订阅信息</strong>
          <div className="toolbar">
            <button className="btn btn-ghost" onClick={load}>{loading ? '刷新中...' : '刷新'}</button>
            <button className="btn btn-primary" onClick={rotateToken}>重置订阅令牌</button>
          </div>
        </div>

        <div className="grid two">
          <div className="field-block">
            <label>订阅链接</label>
            <div className="inline-row">
              <input readOnly value={info?.url || ''} placeholder="暂无订阅链接" />
              <button className="btn btn-ghost" onClick={() => copyText(info?.url || '')}>复制</button>
            </div>
          </div>
          <div className="field-block">
            <label>订阅 Token</label>
            <div className="inline-row">
              <input readOnly value={info?.token || ''} placeholder="暂无 token" />
              <button className="btn btn-ghost" onClick={() => copyText(info?.token || '')}>复制</button>
            </div>
          </div>
        </div>

        <div className="sub-preview-wrap">
          <div>
            <p className="muted">预览链接数量：{preview?.count || 0}</p>
            <textarea readOnly value={preview?.raw || ''} rows={8} />
          </div>
          <div className="qr-box">
            {info?.qrDataUrl ? <img src={info.qrDataUrl} alt="subscription-qr" /> : <span>暂无二维码</span>}
          </div>
        </div>
      </div>

      <div className="panel">
        <div className="head-row">
          <strong>节点链接与用户流量</strong>
        </div>

        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>入站</th>
                <th>节点名</th>
                <th>用户</th>
                <th>最近活跃 IP</th>
                <th>MAC</th>
                <th>最近在线</th>
                <th>上行</th>
                <th>下行</th>
                <th>总流量</th>
                <th>链接</th>
                <th>二维码</th>
              </tr>
            </thead>
            <tbody>
              {nodes.flatMap((inb) =>
                (inb.nodes || []).map((node, idx) => {
                  const stats = statsMap[String(node.statsKey || '').trim()] || null
                  return (
                    <tr key={`${inb.inboundId}-${idx}`}>
                      <td>{inb.remark} ({inb.protocol}:{inb.port})</td>
                      <td>{node.name || '-'}</td>
                      <td>{node.user || '-'}</td>
                      <td>{node.lastActiveIP || node.clientIP || '-'}</td>
                      <td>{node.clientMac || '-'}</td>
                      <td>{node.lastSeen ? new Date(node.lastSeen).toLocaleString() : '-'}</td>
                      <td>{fmtBytes(stats?.uplink)}</td>
                      <td>{fmtBytes(stats?.downlink)}</td>
                      <td>{fmtBytes(stats?.total)}</td>
                      <td>
                        <button className="btn btn-ghost" onClick={() => copyText(node.url || '')}>复制链接</button>
                      </td>
                      <td>
                        {node.qrDataUrl ? <img className="inline-qr" src={node.qrDataUrl} alt="node-qr" /> : '-'}
                      </td>
                    </tr>
                  )
                }),
              )}
              {nodes.length === 0 ? (
                <tr>
                  <td colSpan="11">暂无节点数据</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </div>

      <div className={`hint ${error ? 'err' : ''}`}>{error || message}</div>
    </div>
  )
}
