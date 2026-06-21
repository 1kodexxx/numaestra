import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { categoryApi } from '@entities/category'
import { useCatalog } from '@features/load-catalog'
import { useCreateOrder } from '@features/create-order'
import { EXAMPLE_SONGS } from '@shared/data/examples'
import type { Category, Question, WizardData } from '@entities/category'

const CYAN = '#00e5c0'
const DARK = '#0d0d0d'

function SideCard({ title, onClick }: { title: string; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="w-full rounded-xl text-left cursor-pointer"
      style={{
        background: CYAN, color: DARK, border: 'none',
        padding: '18px 14px', marginBottom: '8px',
        fontWeight: 600, fontSize: '14px',
      }}
      onMouseEnter={(e) => { e.currentTarget.style.filter = 'brightness(1.1)' }}
      onMouseLeave={(e) => { e.currentTarget.style.filter = '' }}
    >
      {title}
    </button>
  )
}

interface ContactModalProps {
  loading: boolean
  error: string | null
  onClose: () => void
  onSubmit: (email: string, phone: string) => void
}

function ContactModal({ loading, error, onClose, onSubmit }: ContactModalProps) {
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [localErr, setLocalErr] = useState('')

  function handleSubmit() {
    if (!email && !phone) { setLocalErr('Укажите email или телефон'); return }
    setLocalErr('')
    onSubmit(email, phone)
  }

  return (
    <div
      className="fixed inset-0 flex items-center justify-center z-50"
      style={{ background: 'rgba(0,0,0,0.75)', backdropFilter: 'blur(4px)' }}
    >
      <div
        className="rounded-2xl p-8 w-full max-w-md"
        style={{ background: '#141414', border: '1px solid #252525' }}
      >
        <div className="text-xl font-bold mb-1">Контактные данные</div>
        <div className="text-sm mb-6" style={{ color: '#777' }}>Отправим готовые треки на ваш email</div>

        <div className="flex flex-col gap-4 mb-6">
          <div>
            <label className="block text-sm mb-1.5" style={{ color: '#777' }}>Email</label>
            <input
              type="email"
              placeholder="your@email.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full rounded-xl outline-none text-sm"
              style={{
                background: '#1c1c1c', border: '1px solid #333',
                padding: '12px 16px', color: '#fff',
              }}
              onFocus={(e) => { e.target.style.borderColor = CYAN }}
              onBlur={(e) => { e.target.style.borderColor = '#333' }}
            />
          </div>
          <div>
            <label className="block text-sm mb-1.5" style={{ color: '#777' }}>Телефон (необязательно)</label>
            <input
              type="tel"
              placeholder="+7 999 000 00 00"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              className="w-full rounded-xl outline-none text-sm"
              style={{
                background: '#1c1c1c', border: '1px solid #333',
                padding: '12px 16px', color: '#fff',
              }}
              onFocus={(e) => { e.target.style.borderColor = CYAN }}
              onBlur={(e) => { e.target.style.borderColor = '#333' }}
            />
          </div>
        </div>

        <div
          className="rounded-xl p-4 text-center mb-6"
          style={{ background: 'rgba(0,229,192,0.1)', border: `1px solid rgba(0,229,192,0.3)` }}
        >
          <div className="font-bold">4 версии песни</div>
          <div className="text-2xl font-extrabold my-1" style={{ color: CYAN }}>2 000 ₽</div>
          <div className="text-xs" style={{ color: '#777' }}>Один платёж, без подписок</div>
        </div>

        {(localErr || error) && (
          <div className="text-sm mb-4 p-3 rounded-lg" style={{ background: '#2a1010', color: '#ef4444', border: '1px solid #4a2020' }}>
            {localErr || error}
          </div>
        )}

        <div className="flex gap-3">
          <button
            onClick={onClose}
            className="flex-1 py-3 rounded-xl text-sm font-semibold"
            style={{ background: 'transparent', border: '1px solid #333', color: '#777', cursor: 'pointer' }}
          >
            Отмена
          </button>
          <button
            onClick={handleSubmit}
            disabled={loading}
            className="flex-1 py-3 rounded-xl text-sm font-semibold"
            style={{
              background: CYAN, color: DARK, border: 'none',
              cursor: loading ? 'wait' : 'pointer',
              opacity: loading ? 0.7 : 1,
            }}
          >
            {loading ? 'Создаём заказ...' : 'Перейти к оплате →'}
          </button>
        </div>
      </div>
    </div>
  )
}

function buildBrief(questions: Question[], answers: Record<string, string>, category: Category): string {
  const parts = questions
    .filter((q) => answers[q.mapping_key]?.trim())
    .map((q) => `${q.question_text}: ${answers[q.mapping_key]}`)
  return parts.length > 0 ? parts.join('. ') : `Персональная песня — ${category.title}`
}

export function CategoryPage() {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const { categories } = useCatalog()
  const [wizard, setWizard] = useState<WizardData | null>(null)
  const [answers, setAnswers] = useState<Record<string, string>>({})
  const [brief, setBrief] = useState('')
  const [brieftouched, setBrieftouched] = useState(false)
  const [showContact, setShowContact] = useState(false)
  const [loadErr, setLoadErr] = useState<string | null>(null)

  const { loading: submitting, error: submitError, submit } = useCreateOrder()
  const category = categories.find((c) => c.id === id)

  useEffect(() => {
    categoryApi
      .wizard(id)
      .then((w) => { setWizard(w); setAnswers({}) })
      .catch((err: Error) => setLoadErr(err.message))
  }, [id])

  // Auto-build brief when answers change (unless user manually edited)
  useEffect(() => {
    if (brieftouched || !wizard || !category) return
    setBrief(buildBrief(wizard.questions, answers, category))
  }, [answers, wizard, category, brieftouched])

  function handleAnswer(key: string, val: string) {
    setAnswers((prev) => ({ ...prev, [key]: val }))
  }

  function handleBriefEdit(val: string) {
    setBrief(val)
    setBrieftouched(true)
  }

  async function handleOrder(email: string, phone: string) {
    const finalBrief = brief.trim() || (category ? `Персональная песня — ${category.title}` : 'Персональная песня')
    await submit({ email, phone, brief: finalBrief, category_id: id, answers })
  }

  const topCategories = categories.slice(0, 6)

  return (
    <div className="flex" style={{ height: 'calc(100vh - 56px)' }}>
      {showContact && (
        <ContactModal
          loading={submitting}
          error={submitError}
          onClose={() => setShowContact(false)}
          onSubmit={handleOrder}
        />
      )}

      {/* Left panel – examples */}
      <aside
        className="flex flex-col p-4"
        style={{ width: '210px', borderRight: '1px solid #252525', flexShrink: 0, overflowY: 'auto' }}
      >
        <div className="flex-1">
          {EXAMPLE_SONGS.map((ex) => (
            <SideCard key={ex.id} title={ex.title} onClick={() => navigate(`/examples/${ex.id}`)} />
          ))}
        </div>
        <p className="text-xs mt-4 text-center" style={{ color: '#555' }}>Список из примеров работ</p>
      </aside>

      {/* Center – brief textarea */}
      <main
        className="flex flex-col p-6 gap-4"
        style={{ flex: 1, overflowY: 'auto' }}
      >
        <button
          onClick={() => navigate('/')}
          className="flex items-center gap-1.5 text-sm w-fit"
          style={{ background: 'none', border: 'none', color: '#777', cursor: 'pointer' }}
          onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
          onMouseLeave={(e) => { e.currentTarget.style.color = '#777' }}
        >
          ← Назад
        </button>

        {category && (
          <div>
            <div className="text-2xl font-bold mb-1">{category.title}</div>
            <div className="text-sm" style={{ color: '#777' }}>{category.description}</div>
          </div>
        )}

        {loadErr && (
          <div className="text-sm p-3 rounded-lg" style={{ background: '#2a1010', color: '#ef4444', border: '1px solid #4a2020' }}>
            {loadErr}
          </div>
        )}

        <textarea
          placeholder="Опишите, какую песню хотите получить. Отвечайте на вопросы справа — текст сформируется автоматически."
          value={brief}
          onChange={(e) => handleBriefEdit(e.target.value)}
          className="flex-1 rounded-2xl resize-none outline-none text-base"
          style={{
            background: '#fff', color: '#0d0d0d',
            padding: '24px', fontSize: '15px',
            border: 'none', lineHeight: 1.7,
            minHeight: '320px',
            boxShadow: '0 4px 32px rgba(0,0,0,0.3)',
          }}
        />

        <button
          onClick={() => setShowContact(true)}
          className="w-full py-4 rounded-2xl text-base font-bold transition-all"
          style={{
            background: CYAN, color: DARK, border: 'none', cursor: 'pointer',
          }}
          onMouseEnter={(e) => { e.currentTarget.style.filter = 'brightness(1.1)' }}
          onMouseLeave={(e) => { e.currentTarget.style.filter = '' }}
        >
          Заказать песню — 2 000 ₽ →
        </button>
      </main>

      {/* Questions panel */}
      <aside
        className="flex flex-col p-5"
        style={{ width: '280px', borderLeft: '1px solid #252525', borderRight: '1px solid #252525', overflowY: 'auto', flexShrink: 0 }}
      >
        <p className="text-xs mb-4 leading-relaxed" style={{ color: '#777' }}>
          При помощи ответов на эти вопросы конструируется промпт
        </p>

        <div className="flex flex-col gap-2 flex-1">
          {wizard ? wizard.questions.map((q) => (
            <div key={q.id}>
              {q.ui_type === 'select' || q.ui_type === 'radio' ? (
                <div>
                  <div className="text-xs mb-1.5" style={{ color: '#aaa' }}>
                    {q.question_text}{q.is_required ? ' *' : ''}
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    {q.options.map((opt) => (
                      <button
                        key={opt.value}
                        onClick={() => handleAnswer(q.mapping_key, opt.value)}
                        className="text-xs px-3 py-1.5 rounded-full transition-all"
                        style={{
                          background: answers[q.mapping_key] === opt.value ? CYAN : '#1c1c1c',
                          color: answers[q.mapping_key] === opt.value ? DARK : '#aaa',
                          border: `1px solid ${answers[q.mapping_key] === opt.value ? CYAN : '#333'}`,
                          cursor: 'pointer',
                          fontWeight: answers[q.mapping_key] === opt.value ? 700 : 400,
                        }}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                </div>
              ) : (
                <div
                  className="rounded-2xl px-4 py-3"
                  style={{ background: '#fff' }}
                >
                  <div className="text-xs mb-1" style={{ color: '#888' }}>
                    {q.question_text}{q.is_required ? ' *' : ''}
                  </div>
                  <input
                    type="text"
                    placeholder="Ваш ответ..."
                    value={answers[q.mapping_key] ?? ''}
                    onChange={(e) => handleAnswer(q.mapping_key, e.target.value)}
                    className="w-full outline-none text-sm"
                    style={{ background: 'transparent', border: 'none', color: DARK }}
                  />
                </div>
              )}
            </div>
          )) : (
            Array.from({ length: 8 }, (_, i) => (
              <div
                key={i}
                className="rounded-2xl"
                style={{
                  height: '48px', background: '#1c1c1c',
                  animation: `pulse 1.5s ease-in-out ${i * 0.1}s infinite alternate`,
                }}
              />
            ))
          )}
        </div>

        <p className="text-xs mt-4 text-center" style={{ color: '#555' }}>
          Вопросы в зависимости от выбранной категории
        </p>
      </aside>

      {/* Right panel – top categories */}
      <aside
        className="flex flex-col p-4"
        style={{ width: '210px', borderLeft: '1px solid #252525', flexShrink: 0, overflowY: 'auto' }}
      >
        <div className="flex-1">
          {topCategories.map((cat) => (
            <SideCard
              key={cat.id}
              title={cat.title}
              onClick={() => navigate(`/category/${cat.id}`)}
            />
          ))}
        </div>
        <p className="text-xs mt-4 text-center" style={{ color: '#555' }}>Список из топовых категорий</p>
      </aside>
    </div>
  )
}
