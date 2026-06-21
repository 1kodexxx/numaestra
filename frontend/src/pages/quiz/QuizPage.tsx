import { useEffect, useState } from 'react'
import { useNavigate, useParams, useLocation } from 'react-router-dom'
import { categoryApi } from '@entities/category'
import { useCreateOrder } from '@features/create-order'
import { Button, TextField } from '@shared/ui'
import type { Question, WizardData } from '@entities/category'

const ACCENT  = '#00e5c0'
const BORDER  = 'rgba(255,255,255,0.07)'
const TEXT2   = 'rgba(255,255,255,0.5)'
const TEXT3   = 'rgba(255,255,255,0.25)'

export function QuizPage() {
  const { categoryId = '' } = useParams<{ categoryId: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const categoryTitle = (location.state as { title?: string } | null)?.title ?? 'Категория'

  const [wizard, setWizard] = useState<WizardData | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [step, setStep] = useState(0)
  const [answers, setAnswers] = useState<Record<string, string>>({})
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [stepError, setStepError] = useState<string | null>(null)
  const { loading: submitting, error: submitError, submit } = useCreateOrder()

  useEffect(() => {
    categoryApi.wizard(categoryId)
      .then(setWizard)
      .catch((err: Error) => setLoadError(err.message))
  }, [categoryId])

  if (loadError) return (
    <div style={{ maxWidth: 560, margin: '40px auto', padding: '0 24px' }}>
      <div style={{ background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)', borderRadius: '12px', padding: '14px 18px', color: '#ef4444', fontSize: '14px' }}>
        {loadError}
      </div>
    </div>
  )

  if (!wizard) return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 'calc(100vh - 60px)' }}>
      <div className="spin-anim" style={{ width: 36, height: 36, borderRadius: '50%', border: '2px solid rgba(255,255,255,0.07)', borderTopColor: ACCENT }} />
    </div>
  )

  const questions: Question[] = wizard.questions
  const totalSteps = questions.length + 1
  const isContactStep = step === questions.length

  function goNext() {
    if (!isContactStep) {
      const q = questions[step]
      if (q.is_required && !answers[q.mapping_key]?.trim()) {
        setStepError('Пожалуйста, заполните это поле')
        return
      }
    }
    setStepError(null)
    setStep(s => s + 1)
  }

  function goPrev() {
    setStepError(null)
    setStep(s => Math.max(0, s - 1))
  }

  async function handleSubmit() {
    if (!email && !phone) { setStepError('Укажите email или телефон'); return }
    setStepError(null)
    const brief = questions.map(q => answers[q.mapping_key] ? `${q.question_text}: ${answers[q.mapping_key]}` : null).filter(Boolean).join('; ') || `Персональная песня — ${categoryTitle}`
    await submit({ email, phone, brief, category_id: categoryId, answers })
  }

  return (
    <div style={{ maxWidth: 600, margin: '0 auto', padding: '40px 24px 60px' }}>
      {/* Header */}
      <div style={{ marginBottom: '32px' }}>
        <button
          onClick={() => navigate(-1)}
          style={{ background: 'none', border: 'none', color: TEXT2, fontSize: '13px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '6px', padding: 0, marginBottom: '16px', transition: 'color 0.15s' }}
          onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
          onMouseLeave={(e) => { e.currentTarget.style.color = TEXT2 }}
        >
          ← Назад
        </button>

        <div style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', background: 'rgba(0,229,192,0.1)', border: '1px solid rgba(0,229,192,0.2)', borderRadius: '20px', padding: '4px 12px 4px 8px', marginBottom: '12px' }}>
          <div style={{ width: '6px', height: '6px', borderRadius: '50%', background: ACCENT }} />
          <span style={{ fontSize: '12px', fontWeight: 600, color: ACCENT }}>{categoryTitle}</span>
        </div>
        <div style={{ fontSize: '26px', fontWeight: 800, letterSpacing: '-0.03em' }}>
          {isContactStep ? 'Контактные данные' : 'Расскажите о песне'}
        </div>
      </div>

      {/* Progress */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '28px' }}>
        {Array.from({ length: totalSteps }, (_, i) => (
          <div
            key={i}
            style={{
              height: '3px', flex: 1, borderRadius: '2px',
              background: i < step ? ACCENT : i === step ? ACCENT : 'rgba(255,255,255,0.1)',
              opacity: i === step ? 1 : i < step ? 0.8 : 0.4,
              transition: 'all 0.3s',
            }}
          />
        ))}
        <span style={{ fontSize: '12px', color: TEXT3, marginLeft: '8px', whiteSpace: 'nowrap' }}>
          {step + 1} / {totalSteps}
        </span>
      </div>

      {/* Card */}
      <div
        className="fade-in"
        key={step}
        style={{
          background: '#0f0f0f',
          border: `1px solid ${BORDER}`,
          borderRadius: '20px', padding: '32px 28px',
          marginBottom: '16px',
        }}
      >
        {!isContactStep ? (
          <QuestionStep
            question={questions[step]}
            value={answers[questions[step].mapping_key] ?? ''}
            onChange={v => { setAnswers(p => ({ ...p, [questions[step].mapping_key]: v })); setStepError(null) }}
          />
        ) : (
          <div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', marginBottom: '24px' }}>
              <TextField label="Email" type="email" value={email} onChange={setEmail} placeholder="your@email.com" surfaceColor="#0f0f0f" />
              <TextField label="Телефон (необязательно)" type="tel" value={phone} onChange={setPhone} placeholder="+7 999 000 00 00" surfaceColor="#0f0f0f" />
            </div>
            <div style={{
              background: 'rgba(0,229,192,0.06)', border: '1px solid rgba(0,229,192,0.15)',
              borderRadius: '14px', padding: '18px 20px', textAlign: 'center',
            }}>
              <div style={{ fontSize: '13px', color: TEXT2, marginBottom: '6px' }}>4 уникальных версии</div>
              <div style={{ fontSize: '32px', fontWeight: 800, color: ACCENT, letterSpacing: '-0.03em' }}>2 000 ₽</div>
              <div style={{ fontSize: '12px', color: TEXT3, marginTop: '4px' }}>Один платёж, без подписок</div>
            </div>
          </div>
        )}

        {(stepError ?? submitError) && (
          <div style={{ background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)', borderRadius: '10px', padding: '10px 14px', fontSize: '13px', color: '#ef4444', marginTop: '16px' }}>
            {stepError ?? submitError}
          </div>
        )}
      </div>

      {/* Navigation */}
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px' }}>
        <Button variant="outlined" size="lg" onClick={goPrev} disabled={step === 0}>← Назад</Button>

        {!isContactStep ? (
          <Button size="lg" onClick={goNext} style={{ flex: 1 }}>Далее →</Button>
        ) : (
          <Button size="lg" onClick={handleSubmit} loading={submitting} style={{ flex: 1 }}>
            Перейти к оплате →
          </Button>
        )}
      </div>
    </div>
  )
}

function QuestionStep({ question, value, onChange }: { question: Question; value: string; onChange: (v: string) => void }) {
  const isChoice = question.ui_type === 'select' || question.ui_type === 'tags' || question.ui_type === 'radio'

  return (
    <>
      <div style={{ fontSize: '18px', fontWeight: 700, letterSpacing: '-0.01em', marginBottom: '20px', lineHeight: 1.3 }}>
        {question.question_text}
        {question.is_required && <span style={{ color: '#ef4444', marginLeft: '4px' }}>*</span>}
      </div>

      {isChoice ? (
        <div style={{ display: 'flex', flexWrap: question.ui_type === 'radio' ? undefined : 'wrap', flexDirection: question.ui_type === 'radio' ? 'column' : undefined, gap: '8px' }}>
          {question.options.map(opt => {
            const sel = value === opt.value
            return (
              <button
                key={opt.value}
                onClick={() => onChange(opt.value)}
                style={{
                  padding: question.ui_type === 'radio' ? '14px 18px' : '9px 16px',
                  borderRadius: question.ui_type === 'radio' ? '12px' : '20px',
                  background: sel ? 'rgba(0,229,192,0.12)' : 'rgba(255,255,255,0.04)',
                  border: `1px solid ${sel ? 'rgba(0,229,192,0.45)' : BORDER}`,
                  color: sel ? ACCENT : 'rgba(255,255,255,0.6)',
                  fontSize: '14px', fontWeight: sel ? 600 : 400, fontFamily: 'inherit',
                  cursor: 'pointer', transition: 'all 0.15s',
                  textAlign: 'left',
                }}
                onMouseEnter={(e) => { if (!sel) { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.18)'; e.currentTarget.style.color = '#fff' } }}
                onMouseLeave={(e) => { if (!sel) { e.currentTarget.style.borderColor = BORDER; e.currentTarget.style.color = 'rgba(255,255,255,0.6)' } }}
              >
                {opt.label}
              </button>
            )
          })}
        </div>
      ) : (
        <TextField
          label="Ваш ответ"
          value={value}
          onChange={onChange}
          multiline={question.ui_type === 'textarea'}
          rows={4}
          surfaceColor="#0f0f0f"
        />
      )}
    </>
  )
}
