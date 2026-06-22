import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { categoryApi } from '@entities/category'
import { useCatalog } from '@features/load-catalog'
import { useCreateOrder } from '@features/create-order'
import { EXAMPLE_SONGS } from '@shared/data/examples'
import { Button, TextField } from '@shared/ui'
import { ContactModal } from '@widgets/contact-modal'
import { SideItem, PanelHeader, Thumb, PlayOverlay, RankCorner, stockImage } from '@widgets/side-panel'
import { useSeo } from '@shared/lib/seo'
import type { Category, Question, WizardData } from '@entities/category'

const ACCENT = '#00e5c0'
const BORDER = 'rgba(255,255,255,0.07)'
const TEXT2 = 'rgba(255,255,255,0.48)'
const TEXT3 = 'rgba(255,255,255,0.22)'
const SURFACE = '#080808'
const PANEL_W = 240

/* ─── breakpoint hook ─── */
function useBreakpoint() {
  const [w, setW] = useState(typeof window !== 'undefined' ? window.innerWidth : 1200)
  useEffect(() => {
    const fn = () => setW(window.innerWidth)
    window.addEventListener('resize', fn)
    return () => { window.removeEventListener('resize', fn) }
  }, [])
  return { isMobile: w < 640, isTablet: w >= 640 && w < 1024, isDesktop: w >= 1024 }
}

/* ─── chip ─── */
function Chip({ label, selected, onClick }: { label: string; selected: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      style={{
        padding: '9px 15px', borderRadius: '20px', fontFamily: 'inherit',
        background: selected ? 'rgba(0,229,192,0.14)' : 'rgba(255,255,255,0.04)',
        border: `1px solid ${selected ? 'rgba(0,229,192,0.5)' : 'rgba(255,255,255,0.1)'}`,
        color: selected ? ACCENT : 'rgba(255,255,255,0.62)',
        fontSize: '13px', fontWeight: selected ? 600 : 400,
        cursor: 'pointer', transition: 'all 0.15s',
      }}
      onMouseEnter={(e) => { if (!selected) { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.2)'; e.currentTarget.style.color = '#fff' } }}
      onMouseLeave={(e) => { if (!selected) { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.1)'; e.currentTarget.style.color = 'rgba(255,255,255,0.62)' } }}
    >
      {label}
    </button>
  )
}

/* ─── section label ─── */
function Section({ label, hint, required, children }: { label: string; hint?: string; required?: boolean; children: React.ReactNode }) {
  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px', marginBottom: '10px' }}>
        <span style={{ fontSize: '13px', color: 'rgba(255,255,255,0.7)', fontWeight: 600 }}>
          {label}{required && <span style={{ color: '#f87171', marginLeft: '3px' }}>*</span>}
        </span>
        {hint && <span style={{ fontSize: '11px', color: TEXT3 }}>{hint}</span>}
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>{children}</div>
    </div>
  )
}

/* ─── one wizard question → подходящий контрол ─── */
function QuestionField({
  q, textValue, tagValues, onText, onToggleTag,
}: {
  q: Question
  textValue: string
  tagValues: string[]
  onText: (v: string) => void
  onToggleTag: (v: string) => void
}) {
  if (q.ui_type === 'text' || q.ui_type === 'textarea') {
    return (
      <TextField
        label={q.question_text}
        required={q.is_required}
        value={textValue}
        onChange={onText}
        multiline={q.ui_type === 'textarea'}
        rows={3}
        surfaceColor={SURFACE}
      />
    )
  }

  const multi = q.ui_type === 'tags'
  return (
    <Section label={q.question_text} required={q.is_required} hint={multi ? 'можно выбрать несколько' : undefined}>
      {q.options.map((opt) => {
        const sel = multi ? tagValues.includes(opt.value) : textValue === opt.value
        return (
          <Chip
            key={opt.value}
            label={opt.label}
            selected={sel}
            onClick={() => (multi ? onToggleTag(opt.value) : onText(textValue === opt.value ? '' : opt.value))}
          />
        )
      })}
    </Section>
  )
}

/* ─── собрать промпт ─── */
function buildBrief(
  questions: Question[], answers: Record<string, string>, cat: Category | undefined,
  customText: string, extraNotes: string,
): string {
  const parts = questions
    .filter((q) => answers[q.mapping_key]?.trim())
    .map((q) => `${q.question_text}: ${answers[q.mapping_key]}`)
  if (extraNotes.trim()) parts.push(`Дополнительно: ${extraNotes.trim()}`)
  let base = parts.length > 0 ? parts.join('. ') : `Персональная песня — ${cat?.title ?? ''}`
  if (customText.trim()) base += `\nТекст песни (использовать дословно):\n${customText.trim()}`
  return base
}

export function CategoryPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { isMobile, isDesktop } = useBreakpoint()
  const { categories } = useCatalog()
  const [wizard, setWizard] = useState<WizardData | null>(null)
  const [answers, setAnswers] = useState<Record<string, string>>({})   // text / select / radio
  const [tagSel, setTagSel] = useState<Record<string, string[]>>({})    // tags (multi)
  const [customText, setCustomText] = useState('')
  const [extraNotes, setExtraNotes] = useState('')
  const [showContact, setShowContact] = useState(false)
  const { loading: submitting, error: submitError, submit } = useCreateOrder()

  const category = categories.find((c) => c.id === id)

  useSeo({
    title: category ? `${category.title} — заказать песню` : 'Конструктор песни',
    description: category?.description
      || 'Соберите свою песню: повод, настроение, жанр и детали. 4 готовые версии за 24 часа.',
  })

  useEffect(() => {
    setWizard(null); setAnswers({}); setTagSel({}); setCustomText(''); setExtraNotes('')
    categoryApi.wizard(id).then(setWizard).catch(() => {})
  }, [id])

  // Итоговая карта ответов: одиночные + мультивыбор (склеенный запятой).
  const mergedAnswers = useMemo(() => {
    const m: Record<string, string> = { ...answers }
    for (const [k, arr] of Object.entries(tagSel)) if (arr.length) m[k] = arr.join(', ')
    return m
  }, [answers, tagSel])

  const preview = useMemo(
    () => buildBrief(wizard?.questions ?? [], mergedAnswers, category, customText, extraNotes),
    [wizard, mergedAnswers, category, customText, extraNotes],
  )

  function toggleTag(key: string, val: string) {
    setTagSel((prev) => {
      const arr = prev[key] ?? []
      return { ...prev, [key]: arr.includes(val) ? arr.filter((x) => x !== val) : [...arr, val] }
    })
  }

  async function handleOrder(email: string, phone: string) {
    await submit({ email, phone, brief: preview, category_id: id, answers: mergedAnswers })
  }

  const topCats = categories.slice(0, 8)
  const icon = '🎵'

  /* ── контент конструктора (общий для мобайла и десктопа) ── */
  const formBody = (
    <>
      <button
        onClick={() => navigate('/')}
        style={{ background: 'none', border: 'none', color: TEXT2, fontSize: '13px', cursor: 'pointer', padding: 0, marginBottom: '18px', display: 'flex', alignItems: 'center', gap: '6px' }}
        onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
        onMouseLeave={(e) => { e.currentTarget.style.color = TEXT2 }}
      >
        ← Назад к категориям
      </button>

      {/* header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '14px', marginBottom: '8px' }}>
        <span style={{
          flexShrink: 0, width: 48, height: 48, borderRadius: '14px',
          background: 'linear-gradient(135deg, #00e5c0, #00bfa5)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: '24px', boxShadow: '0 6px 18px rgba(0,229,192,0.28)',
        }}>{icon}</span>
        <div>
          <div style={{ fontSize: '11px', fontWeight: 700, color: ACCENT, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
            Конструктор песни
          </div>
          <h1 style={{ fontSize: isMobile ? '22px' : '26px', fontWeight: 800, letterSpacing: '-0.03em', lineHeight: 1.15 }}>
            {category?.title ?? 'Категория'}
          </h1>
        </div>
      </div>
      {category?.description && (
        <p style={{ fontSize: '14px', color: TEXT2, lineHeight: 1.55, marginBottom: '26px' }}>{category.description}</p>
      )}

      {/* questions */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
        {wizard ? (
          wizard.questions.map((q) => (
            <QuestionField
              key={q.id}
              q={q}
              textValue={answers[q.mapping_key] ?? ''}
              tagValues={tagSel[q.mapping_key] ?? []}
              onText={(v) => setAnswers((prev) => ({ ...prev, [q.mapping_key]: v }))}
              onToggleTag={(v) => toggleTag(q.mapping_key, v)}
            />
          ))
        ) : (
          Array.from({ length: 6 }, (_, i) => (
            <div key={i} className="skeleton" style={{ height: '52px', borderRadius: '12px', opacity: 1 - i * 0.12 }} />
          ))
        )}

        {/* universal extras */}
        <TextField
          label="Свой текст песни (по желанию)"
          value={customText}
          onChange={setCustomText}
          multiline
          rows={3}
          placeholder="Строки или припев, которые должны прозвучать дословно..."
          surfaceColor={SURFACE}
        />
        <TextField
          label="Дополнительные пожелания (по желанию)"
          value={extraNotes}
          onChange={setExtraNotes}
          multiline
          rows={2}
          placeholder="Что ещё учесть: имена, отсылки, чего избегать..."
          surfaceColor={SURFACE}
        />

        {/* live preview */}
        <div style={{
          background: 'rgba(0,229,192,0.04)', border: '1px solid rgba(0,229,192,0.18)',
          borderRadius: '14px', padding: '16px 18px',
        }}>
          <div style={{ fontSize: '10px', fontWeight: 700, color: ACCENT, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '10px' }}>
            Готовый промпт для Suno
          </div>
          <pre style={{
            whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontFamily: 'inherit',
            fontSize: '13px', lineHeight: 1.6, color: 'rgba(255,255,255,0.82)', margin: 0,
          }}>
            {preview}
          </pre>
        </div>
      </div>
    </>
  )

  const orderBar = (pad: string) => (
    <div style={{ padding: pad, borderTop: `1px solid ${BORDER}`, background: SURFACE }}>
      <Button size="lg" fullWidth onClick={() => setShowContact(true)}>
        Заказать песню — 2 000 ₽ →
      </Button>
    </div>
  )

  const modal = showContact && (
    <ContactModal loading={submitting} error={submitError} onClose={() => setShowContact(false)} onSubmit={handleOrder} />
  )

  /* ── mobile / tablet ── */
  if (!isDesktop) {
    const p = isMobile ? '16px' : '24px'
    return (
      <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100dvh - 60px)', overflow: 'hidden' }}>
        {modal}
        <div style={{ flex: 1, overflowY: 'auto', minHeight: 0, padding: `20px ${p} 28px`, WebkitOverflowScrolling: 'touch' } as React.CSSProperties}>
          {formBody}
        </div>
        {orderBar(`12px ${p} 18px`)}
      </div>
    )
  }

  /* ── desktop: examples | constructor | categories ── */
  return (
    <div style={{ display: 'flex', height: 'calc(100dvh - 60px)', overflow: 'hidden' }}>
      {modal}

      <aside style={{ width: PANEL_W, flexShrink: 0, borderRight: `1px solid ${BORDER}`, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ flex: 1, overflowY: 'auto', minHeight: 0, padding: '18px 10px 12px' }}>
          <PanelHeader icon="🎧" title="Примеры" sub="Послушайте, как звучит" />
          {EXAMPLE_SONGS.map((ex, i) => (
            <SideItem
              key={ex.id} index={i} title={ex.title} sub={ex.category}
              onClick={() => navigate(`/examples/${ex.id}`)}
              leading={(hovered) => (
                <Thumb src={stockImage(ex.id, 'concert,music')} alt={ex.title} active={hovered}>
                  <PlayOverlay active={hovered} />
                </Thumb>
              )}
            />
          ))}
        </div>
      </aside>

      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ flex: 1, overflowY: 'auto', minHeight: 0, padding: '28px 36px' }}>
          <div style={{ maxWidth: 680, margin: '0 auto' }}>
            {formBody}
          </div>
        </div>
        <div style={{ maxWidth: 680, margin: '0 auto', width: '100%' }}>
          {orderBar('16px 36px 20px')}
        </div>
      </main>

      <aside style={{ width: PANEL_W, flexShrink: 0, borderLeft: `1px solid ${BORDER}`, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ flex: 1, overflowY: 'auto', minHeight: 0, padding: '18px 10px 12px' }}>
          <PanelHeader icon="🔥" title="Популярное" sub="Другие категории" />
          {topCats.map((cat, i) => (
            <SideItem
              key={cat.id} index={i} title={cat.title}
              onClick={() => navigate(`/category/${cat.id}`)}
              leading={(hovered) => (
                <Thumb src={cat.cover_image_url || stockImage(cat.id, 'celebration,party')} alt={cat.title} active={hovered}>
                  <RankCorner n={i + 1} />
                </Thumb>
              )}
            />
          ))}
        </div>
      </aside>
    </div>
  )
}
