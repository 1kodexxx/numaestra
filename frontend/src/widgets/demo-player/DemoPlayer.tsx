import { useCallback, useEffect, useRef, useState } from 'react'
import { theme } from '@shared/lib/theme'
import { useAudioBuffering } from '@shared/lib/useAudioBuffering'

const ACCENT = theme.accent
const ACCENT2 = theme.accent2
const TEXT2 = theme.text2
const TEXT3 = theme.text3

/* ─── precomputed waveform heights (стабильны между рендерами) ─── */
const BARS = Array.from({ length: 46 }, (_, i) =>
  26 + Math.sin(i * 0.5) * 26 + Math.sin(i * 1.3) * 13 + Math.sin(i * 2.7) * 6,
)

function fmt(s: number) {
  if (!isFinite(s) || s <= 0) return '0:00'
  const m = Math.floor(s / 60)
  return `${m}:${Math.floor(s % 60).toString().padStart(2, '0')}`
}

/* Этапы «приготовления» демо — для вовлекающей анимации (косметика). */
const COOK_STAGES = [
  { icon: '🎼', label: 'Подбираем мелодию' },
  { icon: '✍️', label: 'Сочиняем строки' },
  { icon: '🎙️', label: 'Записываем вокал' },
  { icon: '🎚️', label: 'Сводим звук' },
  { icon: '✨', label: 'Ищем самый яркий момент' },
]

/* Демо обычно готово за ~1.5 минуты. У демо нет серверного прогресса, поэтому
 * процент — оценка по времени: экспоненциальный подход к потолку, который НИКОГДА
 * не замирает (см. крип ниже). Это снимает ощущение «зависшей автоматики». */
const DEMO_ESTIMATE_SEC = 90
const DEMO_PCT_CAP = 96

function estimateDemoPct(elapsedSec: number): number {
  const tau = DEMO_ESTIMATE_SEC / 2.5
  return Math.min(92, (1 - Math.exp(-elapsedSec / tau)) * 100)
}

/**
 * Премиальный демо-плеер в фирменном стиле. Показывает эффектный «процесс
 * приготовления» во время генерации и кастомный плеер-фрагмент, когда готово.
 */
export function DemoPlayer({ status, url }: { status?: string; url?: string }) {
  if (status === 'processing') return <DemoCooking />
  if (status === 'ready' && url) return <DemoReady url={url} />
  if (status === 'failed') return <DemoFailed />
  if (status === 'none') return <DemoPending />
  return null
}

/* ═══════════════ оболочка-панель в фирменном стиле ═══════════════ */
function Shell({ children, active }: { children: React.ReactNode; active?: boolean }) {
  return (
    <div
      className="fade-in"
      style={{
        position: 'relative',
        marginTop: '24px',
        borderRadius: '20px',
        overflow: 'hidden',
        background: 'linear-gradient(160deg, #101513 0%, #0c0c0c 60%)',
        border: `1px solid ${active ? 'rgba(0,229,192,0.28)' : 'rgba(255,255,255,0.08)'}`,
        boxShadow: active
          ? '0 0 0 1px rgba(0,229,192,0.06), 0 18px 50px -22px rgba(0,229,192,0.35)'
          : '0 18px 50px -28px rgba(0,0,0,0.8)',
      }}
    >
      {/* Ambient glow */}
      <div
        style={{
          position: 'absolute', inset: 0, pointerEvents: 'none',
          background: 'radial-gradient(ellipse 70% 50% at 50% -10%, rgba(0,229,192,0.10) 0%, transparent 70%)',
        }}
      />
      <div style={{ position: 'relative', padding: '20px 22px' }}>{children}</div>
    </div>
  )
}

function DemoBadge({ label, pulse }: { label: string; pulse?: boolean }) {
  return (
    <span
      className={pulse ? 'progress-pulse' : undefined}
      style={{
        display: 'inline-block',
        fontSize: '10px', fontWeight: 800, letterSpacing: '0.1em', textTransform: 'uppercase',
        color: '#04130f', background: `linear-gradient(135deg, ${ACCENT}, ${ACCENT2})`,
        borderRadius: '6px', padding: '3px 8px',
      }}
    >
      {label}
    </span>
  )
}

/* ═══════════════ состояние: демо ещё не началось (none) ═══════════════ */
// Заказ только что создан, демо-задача вот-вот стартует. Лёгкое сообщение без
// тяжёлой анимации: если демо так и не запустится (лимиты/ёмкость), это не выглядит
// как «генерируем вечно».
function DemoPending() {
  return (
    <Shell>
      <div style={{ display: 'flex', alignItems: 'center', gap: '9px' }}>
        <DemoBadge label="Демо" />
        <span style={{ fontSize: '13px', fontWeight: 600, color: TEXT2 }}>
          Готовим бесплатное демо — оно скоро появится здесь…
        </span>
      </div>
    </Shell>
  )
}

/* ═══════════════ состояние: демо не удалось (failed) ═══════════════ */
// Не пугаем клиента: демо — бонус, а не сам продукт. Подчёркиваем, что полная
// песня всё равно будет после оплаты.
function DemoFailed() {
  return (
    <Shell>
      <div style={{ display: 'flex', alignItems: 'center', gap: '9px', marginBottom: '6px' }}>
        <DemoBadge label="Демо" />
        <span style={{ fontSize: '13px', fontWeight: 700 }}>Демо недоступно</span>
      </div>
      <div style={{ fontSize: '12px', color: TEXT2, lineHeight: 1.5 }}>
        Не удалось подготовить превью, но это не влияет на заказ — после оплаты вы получите{' '}
        <b style={{ color: ACCENT }}>4 полные версии</b> песни.
      </div>
    </Shell>
  )
}

/* ═══════════════ состояние: готовим демо ═══════════════ */
function DemoCooking() {
  const [stage, setStage] = useState(0)
  // Живой таймер: главное «успокаивающее» — пользователь видит, что секунды идут,
  // страница жива и процесс не завис. Частая жалоба «походу завис» — именно из-за
  // тишины без обратной связи во время ожидания Suno.
  const [elapsed, setElapsed] = useState(0)
  // Процент-«крип»: всегда ползёт вперёд (max из time-оценки и +0.4%/сек), никогда
  // не уменьшается и не замирает — это второй независимый сигнал «процесс идёт».
  const [pct, setPct] = useState(5)
  const startRef = useRef(Date.now())

  useEffect(() => {
    const stageTimer = setInterval(() => setStage((s) => (s + 1) % COOK_STAGES.length), 2600)
    const tick = setInterval(() => {
      const sec = Math.floor((Date.now() - startRef.current) / 1000)
      setElapsed(sec)
      setPct((prev) => Math.min(DEMO_PCT_CAP, Math.max(prev + 0.4, estimateDemoPct(sec))))
    }, 1000)
    return () => {
      clearInterval(stageTimer)
      clearInterval(tick)
    }
  }, [])

  const cur = COOK_STAGES[stage]

  // Эскалация ободрения: чем дольше ждём, тем сильнее подчёркиваем, что всё идёт
  // штатно и страница обновится сама. Неизменное «пара минут» спустя 3–4 минуты
  // только усиливает страх зависания — поэтому текст меняется со временем.
  const reassure =
    elapsed < 60
      ? 'Обычно занимает пару минут'
      : elapsed < 150
        ? 'Уже скоро — нейросеть дорабатывает звук…'
        : 'Иногда нужно несколько минут. Мы не зависли — демо появится здесь само, как только будет готово. Обновлять страницу не нужно 🙌'

  return (
    <Shell active>
      <div style={{ display: 'flex', alignItems: 'center', gap: '9px', marginBottom: '18px' }}>
        <DemoBadge label="Готовим демо" pulse />
        <span style={{ fontSize: '13px', fontWeight: 700 }}>Колдуем над вашим фрагментом</span>
        <span
          aria-label="Идёт генерация"
          style={{
            marginLeft: 'auto', flexShrink: 0,
            fontSize: '12px', fontWeight: 700, color: ACCENT,
            fontFamily: 'monospace', fontVariantNumeric: 'tabular-nums',
          }}
        >
          {fmt(elapsed)}
        </span>
      </div>

      {/* «живой» эквалайзер */}
      <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'center', gap: '5px', height: '64px', marginBottom: '18px' }}>
        {Array.from({ length: 13 }, (_, i) => (
          <span
            key={i}
            className="wave-animate"
            style={{
              width: '6px',
              height: `${36 + ((i * 53) % 60)}%`,
              borderRadius: '4px',
              transformOrigin: 'bottom',
              background: `linear-gradient(${ACCENT}, ${ACCENT2})`,
              boxShadow: '0 0 10px rgba(0,229,192,0.35)',
              animationDelay: `${i * 0.08}s`,
              animationDuration: `${0.7 + (i % 4) * 0.12}s`,
            }}
          />
        ))}
      </div>

      {/* текущий этап (слева) + процент (справа) */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', marginBottom: '10px' }}>
        <span
          key={stage}
          className="fade-in"
          style={{ fontSize: '13.5px', fontWeight: 600, color: '#fff', minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
        >
          <span style={{ marginRight: '6px' }}>{cur.icon}</span>{cur.label}…
        </span>
        <span style={{ flexShrink: 0, fontSize: '18px', fontWeight: 800, color: ACCENT, letterSpacing: '-0.02em', fontVariantNumeric: 'tabular-nums' }}>
          {Math.round(pct)}%
        </span>
      </div>

      {/* детерминированный бар — всегда ползёт вперёд, никогда не замирает */}
      <div style={{ height: '6px', borderRadius: 999, overflow: 'hidden', background: 'rgba(255,255,255,0.07)' }}>
        <div
          className="bar-flow"
          style={{ height: '100%', width: `${pct}%`, borderRadius: 999, transition: 'width 0.9s cubic-bezier(0.22,1,0.36,1)' }}
        />
      </div>

      <div
        key={reassure}
        className="fade-in"
        style={{ textAlign: 'center', fontSize: '12px', color: TEXT3, marginTop: '12px', minHeight: '34px', lineHeight: 1.5 }}
      >
        {reassure}
      </div>
    </Shell>
  )
}

/* ═══════════════ состояние: демо готово (кастомный плеер) ═══════════════ */
function DemoReady({ url }: { url: string }) {
  const audioRef = useRef<HTMLAudioElement>(null)
  const [playing, setPlaying] = useState(false)
  const [time, setTime] = useState(0)
  const [dur, setDur] = useState(0)
  const [buffering, setBuffering] = useAudioBuffering(audioRef)

  useEffect(() => {
    const el = audioRef.current
    if (!el) return
    const onMeta = () => setDur(el.duration || 0)
    const onTime = () => setTime(el.currentTime)
    const onEnd = () => { setPlaying(false); setTime(0) }
    const onPlay = () => setPlaying(true)
    const onPause = () => setPlaying(false)
    el.addEventListener('loadedmetadata', onMeta)
    el.addEventListener('timeupdate', onTime)
    el.addEventListener('ended', onEnd)
    el.addEventListener('play', onPlay)
    el.addEventListener('pause', onPause)
    return () => {
      el.removeEventListener('loadedmetadata', onMeta)
      el.removeEventListener('timeupdate', onTime)
      el.removeEventListener('ended', onEnd)
      el.removeEventListener('play', onPlay)
      el.removeEventListener('pause', onPause)
    }
  }, [])

  const toggle = useCallback(() => {
    const el = audioRef.current
    if (!el) return
    if (el.paused) {
      setBuffering(true) // мгновенная обратная связь, пока звук грузится
      el.play().catch(() => setBuffering(false))
    } else {
      el.pause()
    }
  }, [setBuffering])

  const progress = dur > 0 ? time / dur : 0

  const seek = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const el = audioRef.current
    if (!el || !dur) return
    const r = e.currentTarget.getBoundingClientRect()
    const p = Math.max(0, Math.min(1, (e.clientX - r.left) / r.width))
    el.currentTime = p * dur
    setTime(p * dur)
  }, [dur])

  return (
    <Shell active={playing}>
      <audio
        ref={audioRef}
        src={url}
        preload="auto"
        controlsList="nodownload noplaybackrate"
        onContextMenu={(e) => e.preventDefault()}
      />

      {/* header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '9px', marginBottom: '3px' }}>
        <DemoBadge label="Демо" />
        <span style={{ fontSize: '13px', fontWeight: 700 }}>Послушайте фрагмент бесплатно</span>
      </div>
      <div style={{ fontSize: '11px', color: TEXT3, marginBottom: '16px' }}>
        Самый яркий момент песни · с тихим водяным знаком
      </div>

      {/* controls + waveform */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
        <button
          type="button"
          onClick={toggle}
          aria-label={playing ? 'Пауза' : 'Воспроизвести'}
          style={{
            flexShrink: 0, width: '52px', height: '52px', borderRadius: '50%', border: 'none', cursor: 'pointer',
            background: `linear-gradient(135deg, ${ACCENT}, ${ACCENT2})`,
            display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#04130f',
            boxShadow: playing
              ? '0 0 0 7px rgba(0,229,192,0.12), 0 8px 22px rgba(0,229,192,0.3)'
              : '0 6px 18px rgba(0,229,192,0.22)',
            transition: 'box-shadow 0.25s, transform 0.15s',
            WebkitTapHighlightColor: 'transparent',
          }}
          onMouseDown={(e) => { e.currentTarget.style.transform = 'scale(0.94)' }}
          onMouseUp={(e) => { e.currentTarget.style.transform = 'scale(1)' }}
          onMouseLeave={(e) => { e.currentTarget.style.transform = 'scale(1)' }}
        >
          {buffering ? (
            <span className="spin-anim" style={{ width: 18, height: 18, borderRadius: '50%', border: '2.5px solid rgba(4,19,15,0.25)', borderTopColor: '#04130f' }} />
          ) : playing ? (
            <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" /></svg>
          ) : (
            <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" style={{ marginLeft: '2px' }}><path d="M8 5v14l11-7z" /></svg>
          )}
        </button>

        {/* clickable waveform */}
        <div
          onClick={seek}
          style={{ flex: 1, display: 'flex', alignItems: 'center', height: '44px', gap: '2px', cursor: 'pointer' }}
        >
          {BARS.map((h, i) => {
            const active = i / BARS.length <= progress
            return (
              <div
                key={i}
                className={playing && active ? 'wave-animate' : undefined}
                style={{
                  flex: 1,
                  height: `${h}%`,
                  minHeight: '4px',
                  borderRadius: '2px',
                  transformOrigin: 'bottom',
                  background: active ? `linear-gradient(${ACCENT}, ${ACCENT2})` : 'rgba(255,255,255,0.09)',
                  boxShadow: active && playing ? '0 0 6px rgba(0,229,192,0.4)' : 'none',
                  animationDelay: `${i * 0.03}s`,
                  transition: 'background 0.12s',
                }}
              />
            )
          })}
        </div>
      </div>

      {/* time */}
      <div style={{
        display: 'flex', justifyContent: 'space-between', marginTop: '10px',
        fontSize: '11px', color: TEXT3, fontFamily: 'monospace', fontVariantNumeric: 'tabular-nums',
      }}>
        <span style={{ color: playing ? ACCENT : TEXT3 }}>{fmt(time)}</span>
        <span>{fmt(dur)}</span>
      </div>

      {/* CTA — закрытие воронки */}
      <div style={{
        marginTop: '14px', padding: '11px 15px', borderRadius: '14px', lineHeight: 1.5,
        background: 'linear-gradient(135deg, rgba(0,229,192,0.10), rgba(0,191,165,0.03))',
        border: '1px solid rgba(0,229,192,0.22)', fontSize: '12px', color: TEXT2,
      }}>
        💎 Нравится? После оплаты — <b style={{ color: ACCENT }}>4 полные версии</b> песни,
        без водяного знака и со скачиванием. Оплата ниже ↓
      </div>
    </Shell>
  )
}
