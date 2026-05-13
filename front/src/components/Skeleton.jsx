import React from 'react'

export default function Skeleton({ width = '100%', height = 16, style = {}, className = '' }) {
  return (
    <div
      className={`skeleton ${className}`}
      style={{ width, height, ...style }}
    />
  )
}
