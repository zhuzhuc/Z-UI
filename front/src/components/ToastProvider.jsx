import React from 'react'

const ToastContext = React.createContext(null)

let _toastId = 0

export function ToastProvider({ children }) {
  const [toasts, dispatch] = React.useReducer((state, action) => {
    switch (action.type) {
      case 'add':
        return [...state.slice(-2), { id: ++_toastId, ...action.payload }]
      case 'remove':
        return state.filter(t => t.id !== action.id)
      default:
        return state
    }
  }, [])

  const toast = React.useCallback((message, type = 'info', duration = 4000) => {
    const id = ++_toastId
    dispatch({ type: 'add', payload: { message, type, id } })
    if (duration > 0) {
      setTimeout(() => dispatch({ type: 'remove', id }), duration)
    }
    return id
  }, [])

  const api = React.useMemo(() => ({
    success: (msg, dur) => toast(msg, 'success', dur),
    error: (msg, dur) => toast(msg, 'error', dur),
    info: (msg, dur) => toast(msg, 'info', dur),
    warning: (msg, dur) => toast(msg, 'warning', dur),
  }), [toast])

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div className="toast-container" role="alert" aria-live="polite">
        {toasts.map(t => (
          <div key={t.id} className={`toast toast-${t.type}`} onClick={() => dispatch({ type: 'remove', id: t.id })}>
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  const ctx = React.useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
