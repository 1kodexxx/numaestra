/** Яндекс.Метрика: инициализация и цели воронки. ID — VITE_YM_COUNTER_ID. */

export const GOALS = {
  BUILDER_OPEN: 'builder_open',
  CONTACT_OPEN: 'contact_open',
  ORDER_SUBMIT: 'order_submit',
  PAYMENT_SUCCESS: 'payment_success',
  PAYMENT_FAIL: 'payment_fail',
  ORDER_COMPLETED: 'order_completed',
  SHARE: 'share_click',
} as const

type YmFn = ((counterId: number, method: string, ...args: unknown[]) => void) & {
  a?: unknown[][]
  l?: number
}

declare global {
  interface Window {
    ym?: YmFn
  }
}

let loaded = false

function counterId(): number | null {
  const raw = import.meta.env.VITE_YM_COUNTER_ID
  if (!raw) return null
  const n = Number(raw)
  return Number.isFinite(n) && n > 0 ? n : null
}

/** Подключает tag.js один раз (без дублирования при HMR). */
export function loadMetrika(): void {
  const id = counterId()
  if (!id || loaded || typeof window === 'undefined') return
  loaded = true

  const w = window
  w.ym =
    w.ym ||
    function (...args: unknown[]) {
      w.ym!.a = w.ym!.a || []
      w.ym!.a!.push(args)
    }
  w.ym!.l = Date.now()

  const s = document.createElement('script')
  s.async = true
  s.src = 'https://mc.yandex.ru/metrika/tag.js'
  document.head.appendChild(s)

  w.ym(id, 'init', {
    clickmap: true,
    trackLinks: true,
    accurateTrackBounce: true,
    webvisor: true,
  })
}

export function reachGoal(goal: string, params?: Record<string, unknown>): void {
  const id = counterId()
  if (!id || !window.ym) return
  window.ym(id, 'reachGoal', goal, params)
}

export function hitPage(path: string): void {
  const id = counterId()
  if (!id || !window.ym) return
  window.ym(id, 'hit', path, { title: document.title })
}
