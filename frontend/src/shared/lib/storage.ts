const ORDER_ID_KEY = 'numaestra_order_id'
const ACCESS_TOKEN_KEY = 'numaestra_access_token'
const INVOICE_MAP_KEY = 'numaestra_invoice_map'
const IDEMPOTENCY_KEY = 'numaestra_idempotency_key'

export const orderStorage = {
  saveOrder(id: string, token: string) {
    localStorage.setItem(ORDER_ID_KEY, id)
    localStorage.setItem(ACCESS_TOKEN_KEY, token)
  },
  saveInvoiceOrder(invoiceId: number, orderId: string) {
    const raw = localStorage.getItem(INVOICE_MAP_KEY)
    const map: Record<string, string> = raw ? JSON.parse(raw) : {}
    map[String(invoiceId)] = orderId
    localStorage.setItem(INVOICE_MAP_KEY, JSON.stringify(map))
  },
  getOrderIdByInvoice(invoiceId: number): string | null {
    const raw = localStorage.getItem(INVOICE_MAP_KEY)
    if (!raw) return null
    const map: Record<string, string> = JSON.parse(raw)
    return map[String(invoiceId)] ?? null
  },
  getOrderId(): string | null {
    return localStorage.getItem(ORDER_ID_KEY)
  },
  getAccessToken(): string | null {
    return localStorage.getItem(ACCESS_TOKEN_KEY)
  },
  // Idempotency-Key стабильный на сессию: генерируется при первом обращении,
  // сохраняется до успешного создания заказа, потом очищается.
  getOrCreateIdempotencyKey(): string {
    let key = sessionStorage.getItem(IDEMPOTENCY_KEY)
    if (!key) {
      key = crypto.randomUUID()
      sessionStorage.setItem(IDEMPOTENCY_KEY, key)
    }
    return key
  },
  clearIdempotencyKey() {
    sessionStorage.removeItem(IDEMPOTENCY_KEY)
  },

  clear() {
    localStorage.removeItem(ORDER_ID_KEY)
    localStorage.removeItem(ACCESS_TOKEN_KEY)
    localStorage.removeItem(INVOICE_MAP_KEY)
  },
}
