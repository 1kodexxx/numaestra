/* Единые токены и переиспользуемые элементы оформления админки. */

export const A = {
  bg: '#0d0d0d',
  surface: '#141414',
  surface2: '#1b1b1b',
  border: 'rgba(255,255,255,0.08)',
  borderStrong: 'rgba(255,255,255,0.14)',
  accent: '#00e5c0',
  accent2: '#00bfa5',
  txt: '#ffffff',
  txt2: 'rgba(255,255,255,0.55)',
  txt3: 'rgba(255,255,255,0.32)',
  green: '#22c55e',
  amber: '#f59e0b',
  red: '#ef4444',
} as const

type Tone = 'green' | 'amber' | 'red' | 'cyan' | 'muted'

const TONE: Record<Tone, { bg: string; border: string; color: string }> = {
  green: { bg: 'rgba(34,197,94,0.12)', border: 'rgba(34,197,94,0.3)', color: '#4ade80' },
  amber: { bg: 'rgba(245,158,11,0.12)', border: 'rgba(245,158,11,0.3)', color: '#fbbf24' },
  red: { bg: 'rgba(239,68,68,0.12)', border: 'rgba(239,68,68,0.3)', color: '#f87171' },
  cyan: { bg: 'rgba(0,229,192,0.12)', border: 'rgba(0,229,192,0.3)', color: '#00e5c0' },
  muted: { bg: 'rgba(255,255,255,0.05)', border: 'rgba(255,255,255,0.12)', color: 'rgba(255,255,255,0.6)' },
}

export function StatusBadge({ label, tone, dot }: { label: string; tone: Tone; dot?: boolean }) {
  const t = TONE[tone]
  return (
    <span style={{
      display: 'inline-flex', alignItems: 'center', gap: '6px',
      background: t.bg, border: `1px solid ${t.border}`, color: t.color,
      borderRadius: '20px', padding: '4px 11px',
      fontSize: '12px', fontWeight: 600, whiteSpace: 'nowrap',
    }}>
      {dot && <span style={{ width: 6, height: 6, borderRadius: '50%', background: t.color }} />}
      {label}
    </span>
  )
}

const PAYMENT: Record<string, { label: string; tone: Tone }> = {
  pending: { label: 'Ожидает оплаты', tone: 'amber' },
  paid: { label: 'Оплачен', tone: 'green' },
  failed: { label: 'Оплата не прошла', tone: 'red' },
  refunded: { label: 'Возврат', tone: 'muted' },
}
const GENERATION: Record<string, { label: string; tone: Tone }> = {
  new: { label: 'Новый', tone: 'muted' },
  queued: { label: 'В очереди', tone: 'cyan' },
  processing: { label: 'Генерируется', tone: 'cyan' },
  completed: { label: 'Готов', tone: 'green' },
  failed: { label: 'Ошибка', tone: 'red' },
}

export function paymentBadge(status: string) {
  const b = PAYMENT[status] ?? { label: status, tone: 'muted' as Tone }
  return <StatusBadge label={b.label} tone={b.tone} />
}
export function generationBadge(status: string) {
  const b = GENERATION[status] ?? { label: status, tone: 'muted' as Tone }
  return <StatusBadge label={b.label} tone={b.tone} dot />
}

/* Заголовок страницы с опциональным действием справа. */
export function PageHeader({ title, subtitle, action }: { title: string; subtitle?: string; action?: React.ReactNode }) {
  return (
    <div className="admin-page-header">
      <div>
        <h1>{title}</h1>
        {subtitle && <div className="admin-page-header__subtitle">{subtitle}</div>}
      </div>
      {action && <div className="admin-page-header__action">{action}</div>}
    </div>
  )
}

/* Карточка-поверхность. */
export function Panel({ children, style, className }: { children: React.ReactNode; style?: React.CSSProperties; className?: string }) {
  return (
    <div
      className={className}
      style={{
        background: A.surface, border: `1px solid ${A.border}`,
        borderRadius: '16px', boxShadow: 'var(--elevation-1)',
        ...style,
      }}
    >
      {children}
    </div>
  )
}

/* Двухколоночная сетка формы — на мобильных одна колонка. */
export function Grid2({ children, gap, className }: { children: React.ReactNode; gap?: string; className?: string }) {
  return (
    <div className={['admin-grid-2', className].filter(Boolean).join(' ')} style={gap ? { gap } : undefined}>
      {children}
    </div>
  )
}

export function ErrorBanner({ children }: { children: React.ReactNode }) {
  return (
    <div style={{
      background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.28)',
      borderRadius: '12px', padding: '12px 16px', fontSize: '14px', color: '#f87171', marginBottom: '16px',
    }}>
      {children}
    </div>
  )
}

export function SuccessBanner({ children }: { children: React.ReactNode }) {
  return (
    <div style={{
      background: 'rgba(34,197,94,0.1)', border: '1px solid rgba(34,197,94,0.28)',
      borderRadius: '12px', padding: '12px 16px', fontSize: '14px', color: '#4ade80',
    }}>
      {children}
    </div>
  )
}

export function EmptyState({ icon, text }: { icon: string; text: string }) {
  return (
    <div style={{ textAlign: 'center', padding: '56px 20px', color: A.txt2 }}>
      <div style={{ fontSize: '40px', marginBottom: '12px', opacity: 0.85 }}>{icon}</div>
      <div style={{ fontSize: '14px' }}>{text}</div>
    </div>
  )
}

/* Поле формы с подписью (для select / number / checkbox, которые не покрывает TextField). */
export function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label style={{ display: 'flex', flexDirection: 'column', gap: '7px' }}>
      <span style={{ fontSize: '12px', color: A.txt2, fontWeight: 500 }}>{label}</span>
      {children}
    </label>
  )
}

/* Нативный select в стиле админки. */
export function Select({ value, onChange, children, style }: {
  value: string; onChange: (v: string) => void; children: React.ReactNode; style?: React.CSSProperties
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      style={{
        background: A.surface2, border: `1px solid ${A.border}`, borderRadius: '12px',
        padding: '12px 14px', color: A.txt, fontSize: '14px', fontFamily: 'inherit',
        outline: 'none', cursor: 'pointer', appearance: 'none',
        ...style,
      }}
    >
      {children}
    </select>
  )
}
