import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { categoryApi } from '@entities/category'
import { useCatalog } from '@features/load-catalog'
import { useCreateOrder } from '@features/create-order'
import { EXAMPLE_SONGS } from '@shared/data/examples'
import type { Category, Question, WizardData } from '@entities/category'

const ACCENT = '#00e5c0'
const SURFACE = '#111111'
const BORDER = 'rgba(255,255,255,0.07)'
const TEXT2 = 'rgba(255,255,255,0.48)'
const TEXT3 = 'rgba(255,255,255,0.22)'
const PANEL_W = 210

/* ── side list item ── */
function SideItem({ title, sub, onClick }: { title: string; sub?: string; onClick: () => void }) {
  const [h, setH] = useState(false)
  return (
    <button
      onClick={onClick}
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
      style={{
        width: '100%', display: 'block', textAlign: 'left',
        background: h ? '#161616' : 'transparent',
        border: 'none', borderRadius: '10px',
        padding: '11px 14px', marginBottom: '2px',
        cursor: 'pointer', transition: 'background 0.15s', position: 'relative',
      }}
    >
      {h && <span style={{ position: 'absolute', left: 0, top: '20%', height: '60%', width: '2px', borderRadius: '1px', background: ACCENT }} />}
      <div style={{ fontSize: '13px', fontWeight: 600, color: h ? '#fff' : TEXT2, transition: 'color 0.15s', lineHeight: 1.3 }}>{title}</div>
      {sub && <div style={{ fontSize: '11px', color: TEXT3, marginTop: '2px' }}>{sub}</div>}
    </button>
  )
}

/* ── question widget ── */
function QuestionWidget({ q, value, onChange }: { q: Question; value: string; onChange: (v: string) => void }) {
  if (q.ui_type === 'select' || q.ui_type === 'radio' || q.ui_type === 'tags') {
    return (
      <div style={{ marginBottom: '18px' }}>
        <div style={{ fontSize: '12px', color: TEXT2, marginBottom: '8px', fontWeight: 500 }}>
          {q.question_text}{q.is_required ? ' *' : ''}
        </div>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
          {q.options.map((opt) => {
            const sel = value === opt.value
            return (
              <button
                key={opt.value}
                onClick={() => onChange(opt.value)}
                style={{
                  padding: '6px 12px', borderRadius: '20px',
                  fontSize: '12px', fontWeight: sel ? 600 : 400, fontFamily: 'inherit',
                  background: sel ? 'rgba(0,229,192,0.14)' : 'rgba(255,255,255,0.04)',
                  border: `1px solid ${sel ? 'rgba(0,229,192,0.5)' : 'rgba(255,255,255,0.08)'}`,
                  color: sel ? ACCENT : TEXT2,
                  cursor: 'pointer', transition: 'all 0.15s',
                }}
                onMouseEnter={(e) => { if (!sel) { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.18)'; e.currentTarget.style.color = '#fff' } }}
                onMouseLeave={(e) => { if (!sel) { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.08)'; e.currentTarget.style.color = TEXT2 } }}
              >
                {opt.label}
              </button>
            )
          })}
        </div>
      </div>
    )
  }

  return (
    <div style={{ marginBottom: '14px' }}>
      <label style={{ display: 'block', fontSize: '12px', color: TEXT2, marginBottom: '6px', fontWeight: 500 }}>
        {q.question_text}{q.is_required ? ' *' : ''}
      </label>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="Ваш ответ..."
        style={{
          width: '100%', background: 'rgba(255,255,255,0.04)',
          border: '1px solid rgba(255,255,255,0.08)',
          borderRadius: '10px', padding: '10px 13px',
          color: '#fff', fontSize: '13px', fontFamily: 'inherit', outline: 'none',
          transition: 'border-color 0.15s',
        }}
        onFocus={(e) => { e.target.style.borderColor = 'rgba(0,229,192,0.4)' }}
        onBlur={(e) => { e.target.style.borderColor = 'rgba(255,255,255,0.08)' }}
      />
    </div>
  )
}

/* ── contact modal ── */
function ContactModal({
  loading, error, onClose, onSubmit,
}: { loading: boolean; error: string | null; onClose: () => void; onSubmit: (email: string, phone: string) => void }) {
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [err, setErr] = useState('')

  function go() {
    if (!email && !phone) { setErr('Укажите email или телефон'); return }
    setErr('')
    onSubmit(email, phone)
  }

  const inputStyle = (focused: boolean): React.CSSProperties => ({
    width: '100%', background: 'rgba(255,255,255,0.04)',
    border: `1px solid ${focused ? 'rgba(0,229,192,0.45)' : 'rgba(255,255,255,0.1)'}`,
    borderRadius: '12px', padding: '13px 16px',
    color: '#fff', fontSize: '14px', fontFamily: 'inherit', outline: 'none',
    transition: 'border-color 0.15s',
  })

  const [em1, setEm1] = useState(false)
  const [em2, setEm2] = useState(false)

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 100,
      background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(6px)',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
    }}>
      <div
        className="scale-in"
        style={{
          background: '#0f0f0f',
          border: '1px solid rgba(255,255,255,0.08)',
          borderRadius: '24px', padding: '36px 32px',
          width: '100%', maxWidth: '420px',
          boxShadow: '0 32px 80px rgba(0,0,0,0.6)',
        }}
      >
        <div style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '6px' }}>
          Оформление заказа
        </div>
        <div style={{ fontSize: '14px', color: TEXT2, marginBottom: '28px' }}>
          Отправим готовые треки на вашу почту
        </div>

        <div style={{ marginBottom: '14px' }}>
          <label style={{ display: 'block', fontSize: '12px', color: TEXT2, marginBottom: '8px', fontWeight: 500 }}>
            Email
          </label>
          <input
            type="email" placeholder="your@email.com"
            value={email} onChange={(e) => setEmail(e.target.value)}
            style={inputStyle(em1)}
            onFocus={() => setEm1(true)} onBlur={() => setEm1(false)}
          />
        </div>

        <div style={{ marginBottom: '24px' }}>
          <label style={{ display: 'block', fontSize: '12px', color: TEXT2, marginBottom: '8px', fontWeight: 500 }}>
            Телефон <span style={{ color: TEXT3 }}>(необязательно)</span>
          </label>
          <input
            type="tel" placeholder="+7 999 000 00 00"
            value={phone} onChange={(e) => setPhone(e.target.value)}
            style={inputStyle(em2)}
            onFocus={() => setEm2(true)} onBlur={() => setEm2(false)}
          />
        </div>

        <div style={{
          background: 'rgba(0,229,192,0.07)', border: '1px solid rgba(0,229,192,0.18)',
          borderRadius: '14px', padding: '16px 20px',
          textAlign: 'center', marginBottom: '20px',
        }}>
          <div style={{ fontSize: '13px', color: TEXT2, marginBottom: '4px' }}>4 уникальных версии</div>
          <div style={{ fontSize: '28px', fontWeight: 800, color: ACCENT, letterSpacing: '-0.03em' }}>2 000 ₽</div>
          <div style={{ fontSize: '12px', color: TEXT3, marginTop: '2px' }}>Один платёж, без подписок</div>
        </div>

        {(err || error) && (
          <div style={{
            background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)',
            borderRadius: '10px', padding: '10px 14px',
            fontSize: '13px', color: '#ef4444', marginBottom: '14px',
          }}>
            {err || error}
          </div>
        )}

        <div style={{ display: 'flex', gap: '10px' }}>
          <button
            onClick={onClose}
            style={{
              flex: 1, padding: '14px', borderRadius: '14px',
              background: 'transparent', border: '1px solid rgba(255,255,255,0.1)',
              color: TEXT2, fontSize: '14px', fontWeight: 600, fontFamily: 'inherit',
              cursor: 'pointer', transition: 'all 0.15s',
            }}
            onMouseEnter={(e) => { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.2)'; e.currentTarget.style.color = '#fff' }}
            onMouseLeave={(e) => { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.1)'; e.currentTarget.style.color = TEXT2 }}
          >
            Отмена
          </button>
          <button
            onClick={go}
            disabled={loading}
            style={{
              flex: 2, padding: '14px', borderRadius: '14px',
              background: 'linear-gradient(135deg, #00e5c0, #00bfa5)',
              border: 'none', color: '#080808',
              fontSize: '14px', fontWeight: 700, fontFamily: 'inherit',
              cursor: loading ? 'wait' : 'pointer',
              opacity: loading ? 0.7 : 1, transition: 'opacity 0.15s',
            }}
          >
            {loading ? 'Создаём заказ...' : 'К оплате →'}
          </button>
        </div>
      </div>
    </div>
  )
}

/* ── page ── */
function buildBrief(questions: Question[], answers: Record<string, string>, cat: Category): string {
  const parts = questions.filter(q => answers[q.mapping_key]?.trim()).map(q => `${q.question_text}: ${answers[q.mapping_key]}`)
  return parts.length > 0 ? parts.join('. ') : `Персональная песня — ${cat.title}`
}

export function CategoryPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { categories } = useCatalog()
  const [wizard, setWizard] = useState<WizardData | null>(null)
  const [answers, setAnswers] = useState<Record<string, string>>({})
  const [brief, setBrief] = useState('')
  const [briefTouched, setBriefTouched] = useState(false)
  const [showContact, setShowContact] = useState(false)
  const { loading: submitting, error: submitError, submit } = useCreateOrder()

  const category = categories.find(c => c.id === id)

  useEffect(() => {
    categoryApi.wizard(id)
      .then(w => { setWizard(w); setAnswers({}) })
      .catch(() => {})
  }, [id])

  useEffect(() => {
    if (briefTouched || !wizard || !category) return
    setBrief(buildBrief(wizard.questions, answers, category))
  }, [answers, wizard, category, briefTouched])

  async function handleOrder(email: string, phone: string) {
    const finalBrief = brief.trim() || `Персональная песня — ${category?.title ?? ''}`
    await submit({ email, phone, brief: finalBrief, category_id: id, answers })
  }

  const topCats = categories.slice(0, 8)

  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 60px)', overflow: 'hidden' }}>
      {showContact && (
        <ContactModal loading={submitting} error={submitError} onClose={() => setShowContact(false)} onSubmit={handleOrder} />
      )}

      {/* Left: examples */}
      <aside style={{ width: PANEL_W, borderRight: `1px solid ${BORDER}`, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ flex: 1, overflowY: 'auto', padding: '20px 12px 12px' }}>
          {EXAMPLE_SONGS.map(ex => (
            <SideItem key={ex.id} title={ex.title} sub={ex.category} onClick={() => navigate(`/examples/${ex.id}`)} />
          ))}
        </div>
        <div style={{ padding: '12px 16px 16px', borderTop: `1px solid ${BORDER}` }}>
          <span style={{ fontSize: '11px', fontWeight: 600, color: TEXT3, letterSpacing: '0.06em', textTransform: 'uppercase' }}>Примеры работ</span>
        </div>
      </aside>

      {/* Center: brief textarea */}
      <main style={{ flex: 1, display: 'flex', flexDirection: 'column', padding: '24px 28px', overflow: 'hidden' }}>
        <button
          onClick={() => navigate('/')}
          style={{ background: 'none', border: 'none', color: TEXT2, fontSize: '13px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '6px', padding: 0, marginBottom: '16px', width: 'fit-content', transition: 'color 0.15s' }}
          onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
          onMouseLeave={(e) => { e.currentTarget.style.color = TEXT2 }}
        >
          ← Назад
        </button>

        {category && (
          <div style={{ marginBottom: '20px' }}>
            <div style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', background: 'rgba(0,229,192,0.1)', border: '1px solid rgba(0,229,192,0.2)', borderRadius: '20px', padding: '4px 12px 4px 8px', marginBottom: '10px' }}>
              <div style={{ width: '6px', height: '6px', borderRadius: '50%', background: ACCENT }} />
              <span style={{ fontSize: '12px', fontWeight: 600, color: ACCENT }}>{category.title}</span>
            </div>
            <div style={{ fontSize: '24px', fontWeight: 800, letterSpacing: '-0.03em', lineHeight: 1.2 }}>Расскажите о вашей песне</div>
            <div style={{ fontSize: '14px', color: TEXT2, marginTop: '6px' }}>Ответьте на вопросы справа — текст сформируется автоматически</div>
          </div>
        )}

        <textarea
          placeholder="Опишите, какую песню хотите получить..."
          value={brief}
          onChange={(e) => { setBrief(e.target.value); setBriefTouched(true) }}
          style={{
            flex: 1,
            background: 'rgba(255,255,255,0.03)',
            border: '1px solid rgba(255,255,255,0.09)',
            borderRadius: '16px', padding: '20px 22px',
            color: '#fff', fontSize: '15px', lineHeight: 1.7,
            fontFamily: 'inherit', resize: 'none', outline: 'none',
            transition: 'border-color 0.2s, box-shadow 0.2s',
            minHeight: '180px',
          }}
          onFocus={(e) => {
            e.target.style.borderColor = 'rgba(0,229,192,0.35)'
            e.target.style.boxShadow = '0 0 0 4px rgba(0,229,192,0.06)'
          }}
          onBlur={(e) => {
            e.target.style.borderColor = 'rgba(255,255,255,0.09)'
            e.target.style.boxShadow = 'none'
          }}
        />

        <button
          onClick={() => setShowContact(true)}
          style={{
            marginTop: '14px', padding: '16px 24px',
            background: 'linear-gradient(135deg, #00e5c0, #00bfa5)',
            border: 'none', borderRadius: '14px',
            color: '#080808', fontSize: '15px', fontWeight: 700,
            fontFamily: 'inherit', cursor: 'pointer',
            transition: 'all 0.2s cubic-bezier(0.34,1.56,0.64,1)',
            boxShadow: '0 4px 20px rgba(0,229,192,0.2)',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.transform = 'translateY(-1px)'
            e.currentTarget.style.boxShadow = '0 8px 28px rgba(0,229,192,0.3)'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.transform = 'translateY(0)'
            e.currentTarget.style.boxShadow = '0 4px 20px rgba(0,229,192,0.2)'
          }}
        >
          Заказать песню — 2 000 ₽ →
        </button>
      </main>

      {/* Questions panel */}
      <aside style={{ width: 270, borderLeft: `1px solid ${BORDER}`, borderRight: `1px solid ${BORDER}`, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ padding: '20px 16px 0', borderBottom: `1px solid ${BORDER}`, paddingBottom: '14px' }}>
          <div style={{ fontSize: '11px', fontWeight: 600, color: TEXT3, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
            Помощник
          </div>
          <div style={{ fontSize: '12px', color: TEXT2, marginTop: '4px' }}>Ответы формируют промпт</div>
        </div>
        <div style={{ flex: 1, overflowY: 'auto', padding: '16px' }}>
          {wizard ? wizard.questions.map(q => (
            <QuestionWidget
              key={q.id}
              q={q}
              value={answers[q.mapping_key] ?? ''}
              onChange={v => setAnswers(prev => ({ ...prev, [q.mapping_key]: v }))}
            />
          )) : (
            Array.from({ length: 7 }, (_, i) => (
              <div key={i} style={{
                height: '44px', borderRadius: '10px',
                background: 'rgba(255,255,255,0.04)', marginBottom: '12px',
                opacity: 1 - i * 0.1,
              }} />
            ))
          )}
        </div>
        <div style={{ padding: '12px 16px 16px', borderTop: `1px solid ${BORDER}` }}>
          <span style={{ fontSize: '11px', fontWeight: 600, color: TEXT3, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
            По категории: {category?.title ?? '—'}
          </span>
        </div>
      </aside>

      {/* Right: top categories */}
      <aside style={{ width: PANEL_W, borderLeft: `1px solid ${BORDER}`, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ flex: 1, overflowY: 'auto', padding: '20px 12px 12px' }}>
          {topCats.map(cat => (
            <SideItem key={cat.id} title={cat.title} onClick={() => navigate(`/category/${cat.id}`)} />
          ))}
        </div>
        <div style={{ padding: '12px 16px 16px', borderTop: `1px solid ${BORDER}` }}>
          <span style={{ fontSize: '11px', fontWeight: 600, color: TEXT3, letterSpacing: '0.06em', textTransform: 'uppercase' }}>Категории</span>
        </div>
      </aside>
    </div>
  )
}
