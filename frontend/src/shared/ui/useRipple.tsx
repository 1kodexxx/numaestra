import { useState, useCallback } from 'react'

interface RippleSpec {
  key: number
  x: number
  y: number
  size: number
}

/**
 * Material ripple. Spread `onPointerDown` onto a `position:relative; overflow:hidden`
 * element and render `rippleEl` as its last child. `color` controls the ripple tint.
 */
export function useRipple(color = 'currentColor') {
  const [ripples, setRipples] = useState<RippleSpec[]>([])

  const onPointerDown = useCallback((e: React.PointerEvent<HTMLElement>) => {
    const el = e.currentTarget
    const rect = el.getBoundingClientRect()
    const size = Math.max(rect.width, rect.height) * 2
    const x = e.clientX - rect.left - size / 2
    const y = e.clientY - rect.top - size / 2
    setRipples((prev) => [...prev, { key: Date.now() + Math.random(), x, y, size }])
  }, [])

  const rippleEl = (
    <span aria-hidden style={{ position: 'absolute', inset: 0, overflow: 'hidden', borderRadius: 'inherit', pointerEvents: 'none' }}>
      {ripples.map((r) => (
        <span
          key={r.key}
          className="md-ripple"
          onAnimationEnd={() => setRipples((prev) => prev.filter((p) => p.key !== r.key))}
          style={{ left: r.x, top: r.y, width: r.size, height: r.size, color }}
        />
      ))}
    </span>
  )

  return { onPointerDown, rippleEl }
}
