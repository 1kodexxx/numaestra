import { theme } from '@shared/lib/theme'

export function Spinner({ size = 36 }: { size?: number }) {
  return (
    <div
      className="spin-anim"
      role="status"
      aria-label="Загрузка"
      style={{
        display: 'inline-block',
        width: size,
        height: size,
        borderRadius: '50%',
        border: `${Math.max(2, Math.round(size / 12))}px solid rgba(255,255,255,0.1)`,
        borderTopColor: theme.accent,
        margin: '0 auto',
      }}
    />
  )
}
