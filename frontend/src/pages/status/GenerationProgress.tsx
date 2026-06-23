import { useEffect, useRef, useState } from 'react'
import type { GenerationStatus } from '@entities/order'

const ACCENT = '#00e5c0'
const TEXT2 = 'rgba(255,255,255,0.5)'
const TEXT3 = 'rgba(255,255,255,0.28)'

// Оценочная длительность генерации (сек). Бренд обещает «10 минут» — берём её
// как верхнюю оценку. Бар асимптотически приближается, но НЕ достигает 100% до
// фактического completed: Suno не отдаёт гранулярный прогресс, и показывать
// честную оценку вместо фальшивых 100% — правильнее.
const ESTIMATE_SEC = 10 * 60
const CAP = 0.94 // потолок до реального завершения
// τ подобран так, что к ESTIMATE_SEC кривая 1 − e^(−t/τ) ≈ CAP.
const TAU = ESTIMATE_SEC / 2.8

const PHASE_MESSAGES: Record<'queued' | 'processing', string[]> = {
  queued: [
    'Заказ принят — становимся в очередь…',
    'Готовим AI-студию…',
  ],
  processing: [
    '✍️ Пишем текст песни под ваш повод…',
    '🎼 Подбираем мелодию и аранжировку…',
    '🎤 Записываем вокал…',
    '🎚️ Сводим и мастерим трек…',
    '✨ Наводим финальные штрихи…',
  ],
}

function fmtRemaining(sec: number): string {
  if (sec <= 0) return 'почти готово…'
  if (sec > 90) return `осталось ~${Math.ceil(sec / 60)} мин`
  return 'меньше минуты…'
}

/**
 * Непрерывный прогресс генерации между опросами статуса (раз в 10с). Тикает
 * ежесекундно: плавно двигает бар, ETA и ротацию «что сейчас делаем» — чтобы
 * у пользователя было ощущение живой работы, а не зависшего экрана.
 *
 * Якорь времени — paid_at с сервера (переживает перезагрузку и одинаков на всех
 * устройствах). Если его нет — момент монтирования как запасной вариант.
 */
export function GenerationProgress({
  status,
  paidAt,
}: {
  status: GenerationStatus
  paidAt?: string
}) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [])

  const fallbackStart = useRef(Date.now())
  const startMs = paidAt ? new Date(paidAt).getTime() : fallbackStart.current
  const elapsedSec = Math.max(0, (now - startMs) / 1000)

  const completed = status === 'completed'
  const raw = 1 - Math.exp(-elapsedSec / TAU)
  const pct = completed ? 100 : Math.min(CAP, raw) * 100

  const phase: 'queued' | 'processing' = status === 'queued' ? 'queued' : 'processing'
  const msgs = PHASE_MESSAGES[phase]
  const message = completed ? 'Песня готова 🎉' : msgs[Math.floor(elapsedSec / 4) % msgs.length]
  const eta = completed ? 'Готово' : fmtRemaining(ESTIMATE_SEC - elapsedSec)

  return (
    <div style={{ marginTop: '4px' }}>
      {/* «Что сейчас делаем» + ETA */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', marginBottom: '10px' }}>
        <span key={message} className="fade-in" style={{ fontSize: '13px', color: TEXT2, fontWeight: 500, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {message}
        </span>
        <span style={{ flexShrink: 0, fontSize: '12px', color: ACCENT, fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>
          {Math.round(pct)}%
        </span>
      </div>

      {/* Бар */}
      <div style={{ position: 'relative', width: '100%', height: 8, borderRadius: 999, background: 'rgba(255,255,255,0.07)', overflow: 'hidden' }}>
        <div
          className={completed ? '' : 'bar-flow'}
          style={{
            height: '100%', width: `${pct}%`, borderRadius: 999,
            background: completed ? ACCENT : undefined,
            transition: 'width 0.9s cubic-bezier(0.22,1,0.36,1)',
          }}
        />
      </div>

      <div style={{ marginTop: '8px', fontSize: '11px', color: TEXT3, textAlign: 'center' }}>
        {completed ? 'Спасибо за ожидание!' : `${eta} · обычно 4 версии готовы за несколько минут`}
      </div>
    </div>
  )
}
