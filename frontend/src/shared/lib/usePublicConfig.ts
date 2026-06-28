import { useEffect, useState } from 'react'
import { publicConfigApi, type PublicConfig } from '@entities/public-config/api'

const FALLBACK: PublicConfig = {
  price_kopecks: 200_000,
  price_label: '2 000 ₽',
  // Держите в синхроне с domain.CurrentConsentDocVersion: при сбое /public/config
  // фронт отправит эту версию, и она должна совпасть с серверной валидацией.
  consent_doc_version: '2026-06-29',
}

let cached: PublicConfig | null = null
let inflight: Promise<PublicConfig> | null = null

function loadPublicConfig(): Promise<PublicConfig> {
  if (cached) return Promise.resolve(cached)
  if (!inflight) {
    inflight = publicConfigApi
      .get()
      .then((cfg) => {
        cached = cfg
        return cfg
      })
      .catch(() => FALLBACK)
      .finally(() => {
        inflight = null
      })
  }
  return inflight
}

/** Публичные настройки витрины: цена и версия согласия с сервера. */
export function usePublicConfig() {
  const [config, setConfig] = useState<PublicConfig>(cached ?? FALLBACK)

  useEffect(() => {
    let active = true
    void loadPublicConfig().then((cfg) => {
      if (active) setConfig(cfg)
    })
    return () => {
      active = false
    }
  }, [])

  return config
}
