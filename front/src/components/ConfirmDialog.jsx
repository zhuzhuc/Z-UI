import React from 'react'

export default function ConfirmDialog({ open, title, message, onConfirm, onCancel }) {
  if (!open) return null
  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 400 }}>
        {title && <h3>{title}</h3>}
        <p style={{ margin: '16px 0', lineHeight: 1.6 }}>{message}</p>
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
          <button className="btn" onClick={onCancel}>取消</button>
          <button className="btn btn-danger" onClick={onConfirm}>确认</button>
        </div>
      </div>
    </div>
  )
}
