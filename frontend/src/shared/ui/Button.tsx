import { useRipple } from './useRipple'

type Variant = 'filled' | 'tonal' | 'outlined' | 'text'
type Size = 'sm' | 'md' | 'lg'

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  fullWidth?: boolean
  loading?: boolean
  leadingIcon?: React.ReactNode
  trailingIcon?: React.ReactNode
}

const SIZES: Record<Size, { padding: string; font: number; height: number; radius: number }> = {
  sm: { padding: '0 16px', font: 13, height: 36, radius: 18 },
  md: { padding: '0 24px', font: 14, height: 44, radius: 22 },
  lg: { padding: '0 28px', font: 15, height: 52, radius: 26 },
}

export function Button({
  variant = 'filled',
  size = 'md',
  fullWidth = false,
  loading = false,
  leadingIcon,
  trailingIcon,
  children,
  disabled,
  style,
  ...rest
}: ButtonProps) {
  const isDark = variant === 'filled'
  const { onPointerDown, rippleEl } = useRipple(isDark ? 'rgba(0,0,0,0.5)' : 'rgba(0,229,192,0.6)')
  const s = SIZES[size]
  const isDisabled = disabled || loading

  const base: React.CSSProperties = {
    position: 'relative',
    overflow: 'hidden',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: '8px',
    height: s.height,
    minWidth: s.height,
    padding: s.padding,
    width: fullWidth ? '100%' : undefined,
    borderRadius: s.radius,
    fontSize: s.font,
    fontWeight: 700,
    fontFamily: 'inherit',
    letterSpacing: '0.01em',
    cursor: isDisabled ? 'default' : 'pointer',
    opacity: isDisabled ? 0.4 : 1,
    border: 'none',
    transition: 'box-shadow var(--dur-short) var(--ease-standard), background var(--dur-short), transform var(--dur-short) var(--ease-spring)',
    WebkitTapHighlightColor: 'transparent',
    userSelect: 'none',
  }

  const variants: Record<Variant, React.CSSProperties> = {
    filled: {
      background: 'linear-gradient(135deg, #00e5c0, #00bfa5)',
      color: '#062420',
      boxShadow: '0 2px 12px rgba(0,229,192,0.22)',
    },
    tonal: {
      background: 'rgba(0,229,192,0.12)',
      color: '#00e5c0',
    },
    outlined: {
      background: 'transparent',
      color: '#00e5c0',
      border: '1px solid rgba(0,229,192,0.4)',
    },
    text: {
      background: 'transparent',
      color: '#00e5c0',
      padding: '0 12px',
    },
  }

  return (
    <button
      {...rest}
      disabled={isDisabled}
      onPointerDown={isDisabled ? undefined : onPointerDown}
      style={{ ...base, ...variants[variant], ...style }}
      onMouseEnter={(e) => {
        if (isDisabled) return
        if (variant === 'filled') {
          e.currentTarget.style.boxShadow = '0 4px 18px rgba(0,229,192,0.34)'
          e.currentTarget.style.transform = 'translateY(-1px)'
        } else {
          e.currentTarget.style.background = variant === 'outlined' || variant === 'text'
            ? 'rgba(0,229,192,0.08)'
            : 'rgba(0,229,192,0.18)'
        }
        rest.onMouseEnter?.(e)
      }}
      onMouseLeave={(e) => {
        if (isDisabled) return
        e.currentTarget.style.transform = 'translateY(0)'
        e.currentTarget.style.boxShadow = variants[variant].boxShadow as string ?? 'none'
        e.currentTarget.style.background = variants[variant].background as string
        rest.onMouseLeave?.(e)
      }}
    >
      {loading ? (
        <span className="spin-anim" style={{ width: 18, height: 18, borderRadius: '50%', border: '2px solid currentColor', borderTopColor: 'transparent', opacity: 0.8 }} />
      ) : (
        <>
          {leadingIcon}
          {children}
          {trailingIcon}
        </>
      )}
      {rippleEl}
    </button>
  )
}
