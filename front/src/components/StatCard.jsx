import React from 'react'
import { LineChart, Line, ResponsiveContainer } from 'recharts'
import MeteorBackground from './MeteorBackground'

const sparkData = [{ v: 10 }, { v: 25 }, { v: 15 }, { v: 40 }, { v: 22 }, { v: 50 }, { v: 35 }]

export default function StatCard({ title, value, icon: Icon, colorClass }) {
  return (
    <div className="stat-card">
      <MeteorBackground />
      <div className="stat-main">
        <div>
          <p>{title}</p>
          <h3>{value}</h3>
        </div>
        <div className={`stat-icon ${colorClass}`}>
          <Icon size={20} />
        </div>
      </div>
      <div className="stat-spark">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={sparkData}>
            <Line type="monotone" dataKey="v" strokeWidth={2} dot={false} className={colorClass} />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

