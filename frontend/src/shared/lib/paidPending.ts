const PREFIX = 'numaestra:paid_pending:'
const TTL_MS = 30 * 60 * 1000

function key(orderId: string) {
  return PREFIX + orderId
}

/** Помечает заказ как «только что оплаченный» после редиректа с Robokassa SuccessURL. */
export function markPaidPending(orderId: string) {
  try {
    sessionStorage.setItem(key(orderId), String(Date.now()))
  } catch {
    // sessionStorage может быть недоступен в приватном режиме — не критично.
  }
}

export function isPaidPending(orderId: string): boolean {
  try {
    const raw = sessionStorage.getItem(key(orderId))
    if (!raw) return false
    const ts = Number(raw)
    if (!Number.isFinite(ts) || Date.now()-ts > TTL_MS) {
      sessionStorage.removeItem(key(orderId))
      return false
    }
    return true
  } catch {
    return false
  }
}

export function clearPaidPending(orderId: string) {
  try {
    sessionStorage.removeItem(key(orderId))
  } catch {
    // ignore
  }
}
