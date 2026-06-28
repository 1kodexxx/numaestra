import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { grantAnalyticsConsent, hasAnalyticsConsent } from './analytics'

const METRIKA_SELECTOR = 'script[src*="mc.yandex.ru/metrika/tag.js"]'

describe('аналитика и согласие на cookie', () => {
  beforeEach(() => {
    // Нужен валидный ID счётчика, иначе loadMetrika no-op независимо от согласия.
    vi.stubEnv('VITE_YM_COUNTER_ID', '12345')
    localStorage.clear()
    document.querySelectorAll(METRIKA_SELECTOR).forEach((s) => s.remove())
  })

  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('не подключает Метрику без согласия и подключает после', () => {
    // Без согласия: флага нет, скрипт Метрики в DOM отсутствует.
    expect(hasAnalyticsConsent()).toBe(false)
    expect(document.querySelector(METRIKA_SELECTOR)).toBeNull()

    // Согласие сохраняется и подключает Метрику.
    grantAnalyticsConsent()

    expect(hasAnalyticsConsent()).toBe(true)
    expect(localStorage.getItem('numaestra_analytics_consent')).toBe('granted')
    expect(document.querySelector(METRIKA_SELECTOR)).not.toBeNull()
  })
})
