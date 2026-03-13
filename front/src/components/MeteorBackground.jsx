import React from 'react'

export default function MeteorBackground() {
  return (
    <div className="meteor-layer" aria-hidden>
      {Array.from({ length: 3 }).map((_, i) => (
        <span
          key={i}
          className="meteor"
          style={{
            top: `${Math.random() * 100}%`,
            left: `${55 + Math.random() * 45}%`,
            animationDelay: `${Math.random() * 4}s`,
            animationDuration: `${2 + Math.random() * 4}s`,
          }}
        />
      ))}
    </div>
  )
}

