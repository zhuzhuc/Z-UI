import React from 'react'
import { api } from '../shared/api'

function genUUID() {
  if (window.crypto && window.crypto.randomUUID) return window.crypto.randomUUID()
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}

function randomPort() {
  return 20000 + Math.floor(Math.random() * 30000)
}

function randomNodeName() {
  return `node-${Date.now().toString().slice(-6)}`
}

function randomPath() {
  return `/${Math.random().toString(36).slice(2, 8)}`
}

function newForm() {
  const base = randomNodeName()
  return {
    remark: base,
    tag: `in-${base}`,
    protocol: 'vless',
    listen: '0.0.0.0',
    port: randomPort(),
    transport: 'ws',
    security: 'tls',
    enable: true,
    clientId: genUUID(),
    clientEmail: `${base}@z-ui`,
    wsHost: 'www.cloudflare.com',
    wsPath: randomPath(),
    tlsSni: 'www.cloudflare.com',
    totalGB: 0,
    deviceLimit: 0,
    proxyProtocol: false,
    useFallback: false,
    fallbackDest: '',
  }
}

function applyPreset(input, type) {
  if (type === 'low') {
    return {
      ...input,
      protocol: 'vless',
      transport: 'ws',
      security: 'tls',
      wsHost: 'www.cloudflare.com',
      tlsSni: 'www.cloudflare.com',
      wsPath: randomPath(),
      useFallback: false,
      fallbackDest: '',
      proxyProtocol: false,
      clientId: input.clientId || genUUID(),
    }
  }
  return {
    ...input,
    protocol: 'vless',
    transport: 'tcp',
    security: 'tls',
    wsPath: '/ws',
    wsHost: 'www.microsoft.com',
    tlsSni: 'www.microsoft.com',
    useFallback: false,
    fallbackDest: '',
    proxyProtocol: false,
    clientId: input.clientId || genUUID(),
  }
}

function detailToPayload(detail, enableValue) {
  return {
    tag: detail.tag,
    remark: detail.remark,
    protocol: detail.protocol,
    listen: detail.listen,
    port: detail.port,
    totalGB: Number(detail.totalGB || 0),
    deviceLimit: Number(detail.deviceLimit || 0),
    authentication: detail.authentication || '',
    decryption: detail.decryption || '',
    encryption: detail.encryption || '',
    transport: detail.transport || 'tcp',
    security: detail.security || 'none',
    proxyProtocol: !!detail.proxyProtocol,
    settings: JSON.parse(detail.settingsJson || '{}'),
    streamSettings: JSON.parse(detail.streamSettingsJson || '{}'),
    sniffing: JSON.parse(detail.sniffingJson || '{}'),
    fallbacks: JSON.parse(detail.fallbacksJson || '[]'),
    sockopt: JSON.parse(detail.sockoptJson || '{}'),
    httpObfs: JSON.parse(detail.httpObfsJson || '{}'),
    externalProxy: JSON.parse(detail.externalProxyJson || '{}'),
    enable: typeof enableValue === 'boolean' ? enableValue : !!detail.enable,
  }
}

function buildPayload(form) {
  const protocol = form.protocol
  const clientId = form.clientId || genUUID()
  let settings = {}

  if (protocol === 'vless') settings = { decryption: 'none', clients: [{ id: clientId, email: form.clientEmail || '' }] }
  else if (protocol === 'vmess') settings = { clients: [{ id: clientId, email: form.clientEmail || '', alterId: 0 }] }
  else if (protocol === 'trojan') settings = { clients: [{ password: clientId, email: form.clientEmail || '' }] }
  else settings = { method: 'aes-128-gcm', password: clientId }

  const streamSettings = {
    network: form.transport,
    security: form.security,
  }
  if (form.transport === 'ws') {
    streamSettings.wsSettings = {
      path: form.wsPath || '/ws',
      headers: { Host: form.wsHost || 'www.cloudflare.com' },
    }
  }
  if (form.security === 'tls') {
    streamSettings.tlsSettings = {
      serverName: form.tlsSni || form.wsHost || 'www.cloudflare.com',
      alpn: ['h2', 'http/1.1'],
      allowInsecure: false,
    }
  }

  return {
    tag: form.tag,
    remark: form.remark,
    protocol: form.protocol,
    listen: form.listen,
    port: Number(form.port),
    totalGB: Number(form.totalGB || 0),
    deviceLimit: Number(form.deviceLimit || 0),
    authentication: '',
    decryption: protocol === 'vless' ? 'none' : '',
    encryption: '',
    transport: form.transport,
    security: form.security,
    proxyProtocol: !!form.proxyProtocol,
    settings,
    streamSettings,
    sniffing: { enabled: true, destOverride: ['http', 'tls'] },
    fallbacks: form.useFallback && form.fallbackDest ? [{ dest: form.fallbackDest }] : [],
    sockopt: {},
    httpObfs: {},
    externalProxy: {},
    enable: !!form.enable,
  }
}

export default function InboundsModule({ t = (k, f) => f || k, lang = 'zh' }) {
  const [items, setItems] = React.useState([])
  const [loading, setLoading] = React.useState(false)
  const [message, setMessage] = React.useState('')
  const [error, setError] = React.useState('')
  const [query, setQuery] = React.useState('')
  const [open, setOpen] = React.useState(false)
  const [editing, setEditing] = React.useState(null)
  const [form, setForm] = React.useState(newForm())
  const [selectedIds, setSelectedIds] = React.useState([])
  const fileInputRef = React.useRef(null)

  const zh = lang !== 'en'

  async function load() {
    setLoading(true)
    setError('')
    try {
      const res = await api('/inbounds')
      setItems(res.items || [])
      setMessage(zh ? `已加载 ${(res.items || []).length} 条入站` : `Loaded ${(res.items || []).length} inbounds`)
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  React.useEffect(() => {
    load()
  }, [])

  function openCreate(preset = '') {
    setEditing(null)
    const base = newForm()
    setForm(preset ? applyPreset(base, preset) : base)
    setOpen(true)
  }

  async function openEdit(id) {
    try {
      const data = await api(`/inbounds/${id}`)
      const stream = JSON.parse(data.streamSettingsJson || '{}')
      const settings = JSON.parse(data.settingsJson || '{}')
      const fallbacks = JSON.parse(data.fallbacksJson || '[]')
      const first = Array.isArray(settings.clients) ? settings.clients[0] || {} : {}

      setEditing(id)
      setForm({
        remark: data.remark || '',
        tag: data.tag || '',
        protocol: data.protocol || 'vless',
        listen: data.listen || '0.0.0.0',
        port: data.port || randomPort(),
        transport: data.transport || stream.network || 'tcp',
        security: data.security || stream.security || 'none',
        enable: data.enable !== false,
        clientId: first.id || first.password || settings.password || '',
        clientEmail: first.email || settings.email || '',
        wsHost: stream.wsSettings?.headers?.Host || 'www.cloudflare.com',
        wsPath: stream.wsSettings?.path || '/ws',
        tlsSni: stream.tlsSettings?.serverName || 'www.cloudflare.com',
        totalGB: data.totalGB || 0,
        deviceLimit: data.deviceLimit || 0,
        proxyProtocol: !!data.proxyProtocol,
        useFallback: Array.isArray(fallbacks) && fallbacks.length > 0,
        fallbackDest: Array.isArray(fallbacks) && fallbacks[0]?.dest ? String(fallbacks[0].dest) : '',
      })
      setOpen(true)
    } catch (e) {
      setError(e.message)
    }
  }

  async function saveInbound(e) {
    e.preventDefault()
    setError('')
    try {
      const payload = buildPayload(form)
      if (editing) {
        await api(`/inbounds/${editing}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        })
      } else {
        await api('/inbounds', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        })
      }
      setOpen(false)
      setMessage(zh ? (editing ? '更新成功' : '创建成功') : (editing ? 'Updated' : 'Created'))
      await load()
    } catch (err) {
      setError(err.message)
    }
  }

  async function removeItem(id) {
    if (!window.confirm(zh ? '确认删除该入站？' : 'Delete this inbound?')) return
    try {
      await api(`/inbounds/${id}`, { method: 'DELETE' })
      await load()
      setMessage(zh ? '删除成功' : 'Deleted')
    } catch (e) {
      setError(e.message)
    }
  }

  async function toggleEnable(item) {
    try {
      const detail = await api(`/inbounds/${item.id}`)
      const payload = detailToPayload(detail, !item.enable)
      await api(`/inbounds/${item.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      await load()
    } catch (e) {
      setError(e.message)
    }
  }

  async function applyAndRestart() {
    try {
      await api('/xray/apply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ restart: true }),
      })
      setMessage(zh ? '已应用并重启 Xray' : 'Applied and restarted Xray')
    } catch (e) {
      setError(e.message)
    }
  }

  function toggleSelect(id, checked) {
    setSelectedIds((prev) => {
      if (checked) return Array.from(new Set([...prev, id]))
      return prev.filter((one) => one !== id)
    })
  }

  function selectAll(checked) {
    if (!checked) {
      setSelectedIds([])
      return
    }
    setSelectedIds(shown.map((item) => item.id))
  }

  async function batchSetEnable(enableValue) {
    if (selectedIds.length === 0) return
    try {
      await Promise.all(
        selectedIds.map(async (id) => {
          const detail = await api(`/inbounds/${id}`)
          const payload = detailToPayload(detail, enableValue)
          await api(`/inbounds/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          })
        }),
      )
      setMessage(zh ? '批量状态更新成功' : 'Batch status updated')
      setSelectedIds([])
      await load()
    } catch (e) {
      setError(e.message)
    }
  }

  async function batchDelete() {
    if (selectedIds.length === 0) return
    if (!window.confirm(zh ? `确认删除选中的 ${selectedIds.length} 项？` : `Delete ${selectedIds.length} selected items?`)) return
    try {
      await Promise.all(selectedIds.map((id) => api(`/inbounds/${id}`, { method: 'DELETE' })))
      setSelectedIds([])
      setMessage(zh ? '批量删除成功' : 'Batch delete done')
      await load()
    } catch (e) {
      setError(e.message)
    }
  }

  async function exportTemplate(id) {
    try {
      const detail = await api(`/inbounds/${id}`)
      const payload = detailToPayload(detail)
      const template = {
        version: 1,
        exportedAt: new Date().toISOString(),
        payload,
      }
      const blob = new Blob([JSON.stringify(template, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${detail.remark || 'inbound'}-template.json`
      a.click()
      URL.revokeObjectURL(url)
      setMessage(zh ? '模板已导出' : 'Template exported')
    } catch (e) {
      setError(e.message)
    }
  }

  function importTemplateFile(file) {
    const reader = new FileReader()
    reader.onload = () => {
      try {
        const json = JSON.parse(String(reader.result || '{}'))
        const payload = json.payload || json
        const stream = payload.streamSettings || {}
        const settings = payload.settings || {}
        const first = Array.isArray(settings.clients) ? settings.clients[0] || {} : {}
        setEditing(null)
        setForm({
          remark: payload.remark || randomNodeName(),
          tag: payload.tag || `in-${randomNodeName()}`,
          protocol: payload.protocol || 'vless',
          listen: payload.listen || '0.0.0.0',
          port: Number(payload.port || randomPort()),
          transport: payload.transport || stream.network || 'tcp',
          security: payload.security || stream.security || 'none',
          enable: payload.enable !== false,
          clientId: first.id || first.password || settings.password || genUUID(),
          clientEmail: first.email || settings.email || '',
          wsHost: stream.wsSettings?.headers?.Host || 'www.cloudflare.com',
          wsPath: stream.wsSettings?.path || '/ws',
          tlsSni: stream.tlsSettings?.serverName || 'www.cloudflare.com',
          totalGB: Number(payload.totalGB || 0),
          deviceLimit: Number(payload.deviceLimit || 0),
          proxyProtocol: !!payload.proxyProtocol,
          useFallback: Array.isArray(payload.fallbacks) && payload.fallbacks.length > 0,
          fallbackDest: Array.isArray(payload.fallbacks) ? String(payload.fallbacks[0]?.dest || '') : '',
        })
        setOpen(true)
      } catch {
        setError(zh ? '模板 JSON 格式错误' : 'Invalid template JSON')
      }
    }
    reader.readAsText(file)
  }

  const shown = items.filter((item) => {
    const q = query.trim().toLowerCase()
    if (!q) return true
    return [item.id, item.remark, item.protocol, item.port, item.tag].join(' ').toLowerCase().includes(q)
  })

  const allSelected = shown.length > 0 && shown.every((item) => selectedIds.includes(item.id))

  return (
    <div className="panel">
      <div className="head-row">
        <strong>{zh ? '入站列表' : 'Inbounds'}</strong>
        <div className="toolbar">
          <input className="search-input" placeholder={zh ? '搜索 名称/协议/端口/ID' : 'Search by name/protocol/port/ID'} value={query} onChange={(e) => setQuery(e.target.value)} />
          <button className="btn btn-ghost" onClick={load}>{loading ? (zh ? '刷新中...' : 'Refreshing...') : t('refresh', '刷新')}</button>
          <button className="btn btn-ghost" onClick={() => fileInputRef.current?.click()}>{zh ? '导入模板' : 'Import Template'}</button>
          <button className="btn btn-ghost" onClick={() => batchSetEnable(true)}>{zh ? '批量启用' : 'Batch Enable'}</button>
          <button className="btn btn-ghost" onClick={() => batchSetEnable(false)}>{zh ? '批量禁用' : 'Batch Disable'}</button>
          <button className="btn btn-danger" onClick={batchDelete}>{zh ? '批量删除' : 'Batch Delete'}</button>
          <button className="btn btn-primary" onClick={() => openCreate('low')}>{zh ? '低延迟预设' : 'Low Latency Preset'}</button>
          <button className="btn btn-primary" onClick={() => openCreate('privacy')}>{zh ? '隐私预设' : 'Privacy Preset'}</button>
          <button className="btn btn-primary" onClick={() => openCreate()}>{zh ? '新增入站' : 'New Inbound'}</button>
          <button className="btn btn-primary" onClick={applyAndRestart}>{zh ? '应用并重启 Xray' : 'Apply & Restart Xray'}</button>
        </div>
      </div>

      <input
        ref={fileInputRef}
        className="file-hidden"
        type="file"
        accept="application/json"
        onChange={(e) => {
          const file = e.target.files?.[0]
          if (file) importTemplateFile(file)
          e.target.value = ''
        }}
      />

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th><input type="checkbox" checked={allSelected} onChange={(e) => selectAll(e.target.checked)} /></th>
              <th>ID</th><th>{zh ? '名称' : 'Remark'}</th><th>{zh ? '协议' : 'Protocol'}</th><th>{zh ? '端口' : 'Port'}</th><th>{zh ? '流量' : 'Traffic'}</th><th>{zh ? '状态' : 'Status'}</th><th>{zh ? '操作' : 'Actions'}</th>
            </tr>
          </thead>
          <tbody>
            {shown.map((item) => (
              <tr key={item.id}>
                <td><input type="checkbox" checked={selectedIds.includes(item.id)} onChange={(e) => toggleSelect(item.id, e.target.checked)} /></td>
                <td>{item.id}</td>
                <td>{item.remark}</td>
                <td>{item.protocol}</td>
                <td>{item.port}</td>
                <td>{item.usedGB} / {item.totalGB} GB</td>
                <td><span className={`status-pill ${item.enable ? 'ok' : 'bad'}`}>{item.enable ? (zh ? '启用' : 'Enabled') : (zh ? '禁用' : 'Disabled')}</span></td>
                <td>
                  <button className="btn btn-ghost" onClick={() => toggleEnable(item)}>{item.enable ? (zh ? '禁用' : 'Disable') : (zh ? '启用' : 'Enable')}</button>
                  <button className="btn btn-ghost" onClick={() => openEdit(item.id)}>{zh ? '编辑' : 'Edit'}</button>
                  <button className="btn btn-ghost" onClick={() => exportTemplate(item.id)}>{zh ? '导出模板' : 'Export'}</button>
                  <button className="btn btn-danger" onClick={() => removeItem(item.id)}>{zh ? '删除' : 'Delete'}</button>
                </td>
              </tr>
            ))}
            {shown.length === 0 ? <tr><td colSpan="8">{zh ? '暂无数据' : 'No data'}</td></tr> : null}
          </tbody>
        </table>
      </div>

      {open ? (
        <div className="modal-mask" onClick={() => setOpen(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3 className="modal-title">{editing ? (zh ? '编辑入站' : 'Edit Inbound') : (zh ? '新增入站' : 'New Inbound')}</h3>
            <div className="toolbar" style={{ marginBottom: 12 }}>
              <button className="btn btn-ghost" type="button" onClick={() => setForm((prev) => applyPreset(prev, 'low'))}>{zh ? '一键低延迟预设' : 'One-click Low Latency'}</button>
              <button className="btn btn-ghost" type="button" onClick={() => setForm((prev) => applyPreset(prev, 'privacy'))}>{zh ? '一键隐私预设' : 'One-click Privacy'}</button>
              <button className="btn btn-ghost" type="button" onClick={() => setForm((prev) => ({ ...prev, clientId: genUUID() }))}>{zh ? '生成 UUID' : 'Generate UUID'}</button>
              <button className="btn btn-ghost" type="button" onClick={() => setForm((prev) => ({ ...prev, port: randomPort(), wsPath: randomPath(), remark: randomNodeName(), tag: `in-${randomNodeName()}` }))}>{zh ? '自动生成名称/端口/路径' : 'Auto Generate Name/Port/Path'}</button>
            </div>
            <form onSubmit={saveInbound}>
              <div className="grid two">
                <div className="field"><label>{zh ? '名称' : 'Remark'}</label><input value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} required /></div>
                <div className="field"><label>tag</label><input value={form.tag} onChange={(e) => setForm({ ...form, tag: e.target.value })} /></div>
                <div className="field"><label>{zh ? '协议' : 'Protocol'}</label><select value={form.protocol} onChange={(e) => setForm({ ...form, protocol: e.target.value })}><option value="vless">vless</option><option value="vmess">vmess</option><option value="trojan">trojan</option><option value="shadowsocks">shadowsocks</option></select></div>
                <div className="field"><label>{zh ? '端口' : 'Port'}</label><input type="number" min="1" max="65535" value={form.port} onChange={(e) => setForm({ ...form, port: Number(e.target.value || 0) })} required /></div>
                <div className="field"><label>{zh ? '监听' : 'Listen'}</label><input value={form.listen} onChange={(e) => setForm({ ...form, listen: e.target.value })} /></div>
                <div className="field"><label>{zh ? '传输' : 'Transport'}</label><select value={form.transport} onChange={(e) => setForm({ ...form, transport: e.target.value })}><option value="tcp">tcp</option><option value="ws">ws</option><option value="grpc">grpc</option></select></div>
                <div className="field"><label>{zh ? '安全' : 'Security'}</label><select value={form.security} onChange={(e) => setForm({ ...form, security: e.target.value })}><option value="none">none</option><option value="tls">tls</option><option value="reality">reality</option></select></div>
                <div className="field"><label>{zh ? '客户端 ID/密码' : 'Client ID/Password'}</label><input value={form.clientId} onChange={(e) => setForm({ ...form, clientId: e.target.value })} /></div>
                <div className="field"><label>{zh ? '客户端邮箱/备注' : 'Client Email'}</label><input value={form.clientEmail} onChange={(e) => setForm({ ...form, clientEmail: e.target.value })} /></div>
                <div className="field"><label>WS Host</label><input value={form.wsHost} onChange={(e) => setForm({ ...form, wsHost: e.target.value })} /></div>
                <div className="field"><label>WS Path</label><input value={form.wsPath} onChange={(e) => setForm({ ...form, wsPath: e.target.value })} /></div>
                <div className="field"><label>TLS SNI</label><input value={form.tlsSni} onChange={(e) => setForm({ ...form, tlsSni: e.target.value })} /></div>
                <div className="field"><label>{zh ? '总流量 GB' : 'Total GB'}</label><input type="number" min="0" value={form.totalGB} onChange={(e) => setForm({ ...form, totalGB: Number(e.target.value || 0) })} /></div>
                <div className="field"><label>{zh ? '设备限制' : 'Device Limit'}</label><input type="number" min="0" value={form.deviceLimit} onChange={(e) => setForm({ ...form, deviceLimit: Number(e.target.value || 0) })} /></div>
                <div className="field"><label><input type="checkbox" checked={form.enable} onChange={(e) => setForm({ ...form, enable: e.target.checked })} /> {zh ? '启用' : 'Enabled'}</label></div>
                <div className="field"><label><input type="checkbox" checked={form.proxyProtocol} onChange={(e) => setForm({ ...form, proxyProtocol: e.target.checked })} /> Proxy Protocol</label></div>
                <div className="field"><label><input type="checkbox" checked={form.useFallback} onChange={(e) => setForm({ ...form, useFallback: e.target.checked })} /> Fallback</label></div>
                {form.useFallback ? <div className="field"><label>fallback.dest</label><input value={form.fallbackDest} onChange={(e) => setForm({ ...form, fallbackDest: e.target.value })} placeholder="127.0.0.1:80" /></div> : null}
              </div>
              <div className="toolbar" style={{ marginTop: 12 }}>
                <button className="btn btn-primary" type="submit">{zh ? '保存' : 'Save'}</button>
                <button className="btn btn-ghost" type="button" onClick={() => setOpen(false)}>{zh ? '取消' : 'Cancel'}</button>
              </div>
            </form>
          </div>
        </div>
      ) : null}

      <div className={`hint ${error ? 'err' : ''}`}>{error || message}</div>
    </div>
  )
}

