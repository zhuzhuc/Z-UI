import React from 'react'

export default class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error) {
    return { hasError: true, error }
  }

  componentDidCatch(error, errorInfo) {
    console.error('ErrorBoundary caught:', error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{
          display: 'flex', flexDirection: 'column', alignItems: 'center',
          justifyContent: 'center', minHeight: '50vh', gap: 16, padding: 32,
          color: 'var(--text, #e2e8f0)', textAlign: 'center'
        }}>
          <h2 style={{ margin: 0, fontSize: 20 }}>Something went wrong</h2>
          <p style={{ margin: 0, opacity: 0.7, maxWidth: 480 }}>
            {this.state.error?.message || 'An unexpected error occurred'}
          </p>
          <button
            onClick={() => window.location.reload()}
            style={{
              padding: '8px 24px', borderRadius: 8, border: 'none',
              background: 'var(--accent, #6366f1)', color: '#fff',
              cursor: 'pointer', fontSize: 14
            }}
          >
            Reload Page
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
