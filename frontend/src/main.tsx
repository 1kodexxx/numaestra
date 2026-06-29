import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './app/App'
// Self-hosted Inter (вместо Google Fonts): убирает внешний рендер-блокирующий
// запрос → быстрее LCP, работает без доступа к fonts.gstatic. Семейство остаётся
// 'Inter'; подключаем нужные веса с латиницей и кириллицей (через unicode-range
// браузер качает только нужные подмножества). Импорт до global.css — чтобы
// @font-face были определены до использования.
import '@fontsource/inter/400.css'
import '@fontsource/inter/500.css'
import '@fontsource/inter/600.css'
import '@fontsource/inter/700.css'
import '@fontsource/inter/800.css'
import './app/styles/global.css'
import { captureReferral } from '@shared/lib/referral'

captureReferral()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
