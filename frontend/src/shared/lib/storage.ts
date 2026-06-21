const ORDER_ID_KEY = 'numaestra_order_id'
const ACCESS_TOKEN_KEY = 'numaestra_access_token'

export const orderStorage = {
  saveOrder(id: string, token: string) {
    localStorage.setItem(ORDER_ID_KEY, id)
    localStorage.setItem(ACCESS_TOKEN_KEY, token)
  },
  getOrderId(): string | null {
    return localStorage.getItem(ORDER_ID_KEY)
  },
  getAccessToken(): string | null {
    return localStorage.getItem(ACCESS_TOKEN_KEY)
  },
  clear() {
    localStorage.removeItem(ORDER_ID_KEY)
    localStorage.removeItem(ACCESS_TOKEN_KEY)
  },
}
