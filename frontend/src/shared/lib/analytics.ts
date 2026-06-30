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

// Ключ согласия на аналитические cookie. ВАЖНО: сам счётчик Метрики (просмотры,
// цели, карта кликов) грузится по умолчанию — это стандартная веб-аналитика, без
// неё не считается реклама/CAC, а Яндекс.Директ вообще не находит счётчик на сайте.
// За согласием остаётся только Вебвизор — запись сессий, самое чувствительное.
const CONSENT_KEY = 'numaestra_analytics_consent'

/** Дал ли пользователь согласие на аналитические cookie → включает Вебвизор. */
export function hasAnalyticsConsent(): boolean {
  try {
    return localStorage.getItem(CONSENT_KEY) === 'granted'
  } catch {
    return false
  }
}

/** Сохраняет согласие на запись сессий (Вебвизор). Счётчик к этому моменту уже
 *  загружен по умолчанию; если по какой-то причине ещё нет — догружаем здесь.
 *  Вебвизор задаётся в init, поэтому фактически включится со следующего визита. */
export function grantAnalyticsConsent(): void {
  try {
    localStorage.setItem(CONSENT_KEY, 'granted')
  } catch {
    // localStorage недоступен (приватный режим) — согласие проживёт сессию.
  }
  loadMetrika()
}

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
  s.src = 'https://mc.yandex.ru/metrika/tag.js?id=' + id
  document.head.appendChild(s)

  // Опции из официального сниппета счётчика. ssr — HTML отдаётся серверным
  // SEO-инжектором; ecommerce — хук на window.dataLayer для будущих e-commerce
  // событий; clickmap — карта кликов. webvisor (запись сессий) включаем только
  // после cookie-согласия — это самые чувствительные данные; счётчик же работает
  // для всех, иначе Директ не видит его и реклама/CAC не считаются.
  w.ym(id, 'init', {
    ssr: true,
    webvisor: hasAnalyticsConsent(),
    clickmap: true,
    ecommerce: 'dataLayer',
    referrer: document.referrer,
    url: location.href,
    accurateTrackBounce: true,
    trackLinks: true,
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
