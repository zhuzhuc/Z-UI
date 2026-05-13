import React from 'react'

export default function MeteorBackground() {
  const meteors = React.useMemo(() =>
    Array.from({ length: 3 }, () => ({
      top: `${Math.random() * 100}%`,
      left: `${55 + Math.random() * 45}%`,
      animationDelay: `${Math.random() * 4}s`,
      animationDuration: `${2 + Math.random() * 4}s`,
    })), [])

  return (
    <div className="meteor-layer" aria-hidden="true">
      {meteors.map((style, i) => (
        <span key={i} className="meteor" style={style} />
      ))}
    </div>
  )
}
