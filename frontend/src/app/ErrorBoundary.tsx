import { Component, type ReactNode } from 'react'

interface State { hasError: boolean }

/** Ловит ошибки рендера в дереве и показывает дружелюбный фолбэк вместо белого экрана. */
export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: unknown) {
    // eslint-disable-next-line no-console
    console.error('UI error:', error)
  }

  render() {
    if (!this.state.hasError) return this.props.children
    return (
      <div style={{
        minHeight: '100vh', display: 'flex', flexDirection: 'column',
        alignItems: 'center', justifyContent: 'center', gap: '14px',
        padding: '24px', textAlign: 'center', color: '#fff', background: '#080808',
      }}>
        <div style={{ fontSize: '48px' }}>🎧</div>
        <div style={{ fontSize: '20px', fontWeight: 800, letterSpacing: '-0.02em' }}>Что-то пошло не так</div>
        <div style={{ fontSize: '14px', color: 'rgba(255,255,255,0.5)', maxWidth: '320px', lineHeight: 1.5 }}>
          Произошла непредвиденная ошибка. Обновите страницу — обычно это помогает.
        </div>
        <button
          onClick={() => window.location.reload()}
          style={{
            marginTop: '8px', padding: '12px 24px', borderRadius: '14px',
            background: 'linear-gradient(135deg, #00e5c0, #00bfa5)', color: '#062420',
            border: 'none', fontSize: '14px', fontWeight: 700, fontFamily: 'inherit', cursor: 'pointer',
          }}
        >
          Обновить страницу
        </button>
      </div>
    )
  }
}
