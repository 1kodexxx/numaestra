import { useRipple } from './useRipple'

interface IconButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  label: string
  size?: number
}

/** Material icon button with circular state layer + ripple. 48px touch target by default. */
export function IconButton({ label, size = 44, children, disabled, style, ...rest }: IconButtonProps) {
  const { onPointerDown, rippleEl } = useRipple('rgba(255,255,255,0.6)')
  return (
    <button
      {...rest}
      aria-label={label}
      disabled={disabled}
      onPointerDown={disabled ? undefined : onPointerDown}
      className="state-layer"
      style={{
        width: size, height: size,
        display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
        borderRadius: '50%',
        background: 'transparent',
        border: 'none',
        color: 'rgba(255,255,255,0.7)',
        cursor: disabled ? 'default' : 'pointer',
        opacity: disabled ? 0.35 : 1,
        transition: 'color var(--dur-short)',
        WebkitTapHighlightColor: 'transparent',
        ...style,
      }}
    >
      {children}
      {rippleEl}
    </button>
  )
}
