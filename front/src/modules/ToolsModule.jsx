import React from 'react'
import { api } from '../shared/api'

export default function ToolsModule() {
  const [bbr, setBbr] = React.useState(null)
  const [speedURL, setSpeedURL] = React.useState('https://speed.cloudflare.com/__down?bytes=10000000')
  const [speedResult, setSpeedResult] = React.useState(null)
  const [loading, setLoading] = React.useState(false)
  const [message, setMessage] = React.useState('')
  const [error, setError] = React.useState('')

  async function loadBBR() {
    try {
      const res = await api('/tools/bbr')
      setBbr(res)
    } catch (e) {
      setError(e.message)
    }
  }

  React.useEffect(() => {
    loadBBR()
  }, [])

  async function enableBBR() {
    setError('')
    try {
      const res = await api('/tools/bbr/enable', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ qdisc: 'fq', congestion: 'bbr' }),
      })
      setBbr(res.status || null)
      setMessage('BBR 已尝试启用（需 root 且 Linux）')
    } catch (e) {
      setError(e.message)
    }
  }

  async function runSpeedtest() {
    setLoading(true)
    setError('')
    try {
      const res = await api('/tools/speedtest', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: speedURL }),
      })
      setSpeedResult(res)
      setMessage('测速完成')
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="module-stack">
      <div className="panel">
        <div className="head-row">
          <strong>BBR 加速</strong>
          <div className="toolbar">
            <button className="btn btn-ghost" onClick={loadBBR}>刷新状态</button>
            <button className="btn btn-primary" onClick={enableBBR}>启用 BBR</button>
          </div>
        </div>

        <div className="kv-grid">
          <div><span>系统</span><b>{bbr?.os || '-'}</b></div>
          <div><span>是否支持</span><b>{bbr?.supported ? '是' : '否'}</b></div>
          <div><span>是否启用</span><b>{bbr?.enabled ? '已启用' : '未启用'}</b></div>
          <div><span>队列算法</span><b>{bbr?.qdisc || '-'}</b></div>
          <div><span>拥塞控制</span><b>{bbr?.congestionControl || '-'}</b></div>
          <div><span>可用算法</span><b>{bbr?.availableCongestion || '-'}</b></div>
        </div>
      </div>

      <div className="panel">
        <div className="head-row">
          <strong>网络测速</strong>
          <div className="toolbar">
            <button className="btn btn-primary" onClick={runSpeedtest}>{loading ? '测速中...' : '开始测速'}</button>
          </div>
        </div>

        <div className="field-block">
          <label>测速 URL</label>
          <input value={speedURL} onChange={(e) => setSpeedURL(e.target.value)} placeholder="https://speed.cloudflare.com/__down?bytes=10000000" />
        </div>

        <div className="kv-grid">
          <div><span>TCP 延迟</span><b>{speedResult ? `${speedResult.dialMs} ms` : '-'}</b></div>
          <div><span>下载时长</span><b>{speedResult ? `${speedResult.durationMs} ms` : '-'}</b></div>
          <div><span>下载流量</span><b>{speedResult ? `${Number(speedResult.downloadedMB || 0).toFixed(2)} MB` : '-'}</b></div>
          <div><span>速度</span><b>{speedResult ? `${Number(speedResult.speedMbps || 0).toFixed(2)} Mbps` : '-'}</b></div>
          <div><span>上限</span><b>{speedResult ? `${speedResult.cappedAtMB} MB` : '-'}</b></div>
          <div><span>测试时间</span><b>{speedResult?.testedAt ? new Date(speedResult.testedAt).toLocaleString() : '-'}</b></div>
        </div>
      </div>

      <div className={`hint ${error ? 'err' : ''}`}>{error || message}</div>
    </div>
  )
}
