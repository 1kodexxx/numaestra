/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_POLL_INTERVAL_MS?: string
  readonly VITE_YM_COUNTER_ID?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
