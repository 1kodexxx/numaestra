import type { CSSProperties } from 'react'

/** Задержка появления для стаггера списков и сеток. */
export function staggerDelay(index: number, stepMs = 55, capMs = 420): CSSProperties {
  return { animationDelay: `${Math.min(index * stepMs, capMs)}ms` }
}
