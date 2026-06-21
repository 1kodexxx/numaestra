interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  elevation?: 0 | 1 | 2 | 3
  interactive?: boolean
  padding?: string | number
}

const ELEVATION = ['none', 'var(--elevation-1)', 'var(--elevation-2)', 'var(--elevation-3)']

/** Material surface card. `interactive` adds hover elevation + cursor. */
export function Card({ elevation = 1, interactive = false, padding = 24, style, children, ...rest }: CardProps) {
  return (
    <div
      {...rest}
      style={{
        background: '#121212',
        border: '1px solid rgba(255,255,255,0.08)',
        borderRadius: '16px',
        padding,
        boxShadow: ELEVATION[elevation],
        transition: 'box-shadow var(--dur-medium) var(--ease-standard), border-color var(--dur-short), transform var(--dur-short) var(--ease-spring)',
        cursor: interactive ? 'pointer' : undefined,
        ...style,
      }}
      onMouseEnter={interactive ? (e) => {
        e.currentTarget.style.boxShadow = ELEVATION[3]
        e.currentTarget.style.borderColor = 'rgba(255,255,255,0.14)'
        e.currentTarget.style.transform = 'translateY(-2px)'
        rest.onMouseEnter?.(e)
      } : rest.onMouseEnter}
      onMouseLeave={interactive ? (e) => {
        e.currentTarget.style.boxShadow = ELEVATION[elevation]
        e.currentTarget.style.borderColor = 'rgba(255,255,255,0.08)'
        e.currentTarget.style.transform = 'translateY(0)'
        rest.onMouseLeave?.(e)
      } : rest.onMouseLeave}
    >
      {children}
    </div>
  )
}
