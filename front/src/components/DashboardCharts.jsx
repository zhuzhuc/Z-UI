import React from 'react'
import {
  ResponsiveContainer,
  ComposedChart,
  CartesianGrid,
  XAxis,
  YAxis,
  Tooltip,
  Bar,
  Line,
  PieChart,
  Pie,
  Cell,
} from 'recharts'

const defaultHistory = [
  { name: '00:00:00', up: 0, down: 0 },
]

const defaultProtocol = [
  { name: 'vless', value: 1, color: '#10b981' },
]

const palette = ['#8b5cf6', '#10b981', '#3b82f6', '#f59e0b', '#ef4444']

export default function DashboardCharts({ history = defaultHistory, protocolData = defaultProtocol, lang = 'zh' }) {
  const pieData = protocolData.length > 0
    ? protocolData.map((item, idx) => ({ ...item, color: item.color || palette[idx % palette.length] }))
    : defaultProtocol
  const total = pieData.reduce((sum, item) => sum + Number(item.value || 0), 0)

  return (
    <div className="charts-grid">
      <div className="chart-panel big">
        <div className="chart-head">
          <h3>{lang === 'en' ? 'Realtime Resource Trend' : '实时资源趋势'}</h3>
        </div>
        <div className="chart-box">
          <ResponsiveContainer width="100%" height="100%">
            <ComposedChart data={history}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#2a2d39" />
              <XAxis dataKey="name" axisLine={false} tickLine={false} tick={{ fill: '#64748b', fontSize: 10 }} />
              <YAxis axisLine={false} tickLine={false} tick={{ fill: '#64748b', fontSize: 10 }} />
              <Tooltip contentStyle={{ backgroundColor: '#1e212b', border: '1px solid #334155', borderRadius: 8 }} />
              <Bar dataKey="up" fill="#6366f1" radius={[4, 4, 0, 0]} barSize={8} />
              <Line type="monotone" dataKey="down" stroke="#10b981" strokeWidth={2.2} dot={{ r: 2 }} activeDot={{ r: 5 }} />
            </ComposedChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div className="chart-panel">
        <div className="chart-head">
          <h3>{lang === 'en' ? 'Inbound Protocol Ratio' : '入站协议占比'}</h3>
        </div>
        <div className="chart-box pie">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie data={pieData} innerRadius={55} outerRadius={76} paddingAngle={5} dataKey="value">
                {pieData.map((entry, index) => (
                  <Cell key={index} fill={entry.color} stroke="none" />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
          <div className="pie-center">
            <strong>{total}</strong>
            <span>{lang === 'en' ? 'Inbounds' : '入站数'}</span>
          </div>
        </div>
        <div className="pie-legend">
          {pieData.map((item) => {
            const value = Number(item.value || 0)
            const percent = total > 0 ? ((value / total) * 100).toFixed(1) : '0.0'
            return (
              <div key={item.name} className="pie-legend-item">
                <span className="dot" style={{ backgroundColor: item.color }} />
                <span className="name">{String(item.name || '-').toUpperCase()}</span>
                <span className="value">{value} ({percent}%)</span>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
