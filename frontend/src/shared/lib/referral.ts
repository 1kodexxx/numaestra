const REFERRAL_KEY = 'numaestra_ref'

/** Читает ?ref=CODE из URL и сохраняет в localStorage. */
export function captureReferral(): void {
  const params = new URLSearchParams(window.location.search)
  const ref = params.get('ref')?.trim()
  if (ref) {
    localStorage.setItem(REFERRAL_KEY, ref)
  }
}

/** Возвращает сохранённый реферальный код или пустую строку. */
export function getReferralCode(): string {
  return localStorage.getItem(REFERRAL_KEY) ?? ''
}
