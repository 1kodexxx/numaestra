import { useEffect, useRef } from 'react'
import { adminOrderApi } from '@entities/admin-order'
import { useAdminSession } from '@features/admin-session'
import { showToast } from '@shared/ui'

// Опрос новых заказов, пока открыта админка. Не push в полном смысле (работает
// только при открытой вкладке), но без инфраструктуры: звук + браузерное
// уведомление, иначе — тост. Гейт по авторизации (login != null).
const POLL_MS = 25_000

// beep — короткий сигнал через Web Audio (без аудио-файла). Может быть заглушён
// политикой автоплея, пока не было взаимодействия со страницей — тогда просто тихо.
function beep() {
  try {
    const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
    const ctx = new Ctx()
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.connect(gain)
    gain.connect(ctx.destination)
    osc.type = 'sine'
    osc.frequency.value = 880
    gain.gain.setValueAtTime(0.0001, ctx.currentTime)
    gain.gain.exponentialRampToValueAtTime(0.25, ctx.currentTime + 0.02)
    gain.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.4)
    osc.start()
    osc.stop(ctx.currentTime + 0.42)
    osc.onended = () => ctx.close()
  } catch {
    /* аудио недоступно — не критично */
  }
}

function notifyNewOrder(invoiceId: number, subtitle: string) {
  beep()
  const title = `🎵 Новый заказ #${invoiceId}`
  if ('Notification' in window && Notification.permission === 'granted') {
    try {
      const n = new Notification(title, { body: subtitle, tag: `order-${invoiceId}` })
      n.onclick = () => {
        window.focus()
        n.close()
      }
      return
    } catch {
      /* упадём в тост ниже */
    }
  }
  showToast(`${title} — ${subtitle}`)
}

export function NewOrderAlerts() {
  const { login } = useAdminSession()
  const lastInvoiceRef = useRef<number | null>(null)
  const seededRef = useRef(false)

  useEffect(() => {
    if (!login) return

    // Разрешение на уведомления запрашиваем один раз при входе в админку.
    if ('Notification' in window && Notification.permission === 'default') {
      Notification.requestPermission().catch(() => {})
    }

    let stopped = false
    let timer: ReturnType<typeof setTimeout>

    async function poll() {
      try {
        const data = await adminOrderApi.list(1, 5)
        const orders = data.orders ?? []
        if (orders.length > 0) {
          const newest = orders.reduce((a, b) => (b.invoice_id > a.invoice_id ? b : a))
          if (!seededRef.current) {
            // Первый опрос — только запоминаем максимум, не пикаем (иначе алерт
            // срабатывал бы при каждом открытии админки на уже существующие заказы).
            lastInvoiceRef.current = newest.invoice_id
            seededRef.current = true
          } else if (lastInvoiceRef.current != null && newest.invoice_id > lastInvoiceRef.current) {
            lastInvoiceRef.current = newest.invoice_id
            notifyNewOrder(newest.invoice_id, newest.email || newest.phone || newest.brief || 'Открой админку')
          }
        }
      } catch {
        /* сеть/сессия — молча повторим на следующем тике */
      }
      if (!stopped) timer = setTimeout(poll, POLL_MS)
    }

    poll()
    return () => {
      stopped = true
      clearTimeout(timer)
    }
  }, [login])

  return null
}
