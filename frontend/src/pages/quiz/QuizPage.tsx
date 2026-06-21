import { useEffect, useState } from 'react'
import { useNavigate, useParams, useLocation } from 'react-router-dom'
import { categoryApi } from '@entities/category'
import { useCreateOrder } from '@features/create-order'
import { Spinner } from '@shared/ui'
import type { Question, WizardData } from '@entities/category'
import styles from './QuizPage.module.css'

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
  const [plan, setPlan] = useState<'standard' | 'premium'>('standard')
  const [stepError, setStepError] = useState<string | null>(null)

  const { loading: submitting, error: submitError, submit } = useCreateOrder()

  useEffect(() => {
    categoryApi
      .wizard(categoryId)
      .then(setWizard)
      .catch((err: Error) => setLoadError(err.message))
  }, [categoryId])

  if (loadError) return <div className={styles.error}>{loadError}</div>
  if (!wizard) return <div className={styles.center}><Spinner /></div>

  const questions: Question[] = wizard.questions
  const totalSteps = questions.length + 1 // +1 для шага контактов
  const isContactStep = step === questions.length

  function goNext() {
    if (!isContactStep) {
      const q = questions[step]
      const val = answers[q.mapping_key] ?? ''
      if (q.is_required && !val.trim()) {
        setStepError('Пожалуйста, заполните это поле')
        return
      }
    }
    setStepError(null)
    setStep((s) => s + 1)
  }

  function goPrev() {
    setStepError(null)
    setStep((s) => Math.max(0, s - 1))
  }

  function handleAnswer(key: string, value: string) {
    setAnswers((prev) => ({ ...prev, [key]: value }))
    setStepError(null)
  }

  async function handleSubmit() {
    if (!email && !phone) { setStepError('Укажите email или телефон'); return }
    setStepError(null)
    const brief = questions
      .map((q) => answers[q.mapping_key] ? `${q.question_text}: ${answers[q.mapping_key]}` : null)
      .filter(Boolean)
      .join('; ') || 'Персональная песня'

    await submit({ email, phone, brief, plan, category_id: categoryId, answers })
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.back} onClick={() => navigate('/')}>← Назад</button>
        <div className={styles.title}>{categoryTitle}</div>
        <div className={styles.subtitle}>Ответьте на несколько вопросов</div>
      </div>

      <div className={styles.stepIndicator}>
        {Array.from({ length: totalSteps }, (_, i) => (
          <div
            key={i}
            className={`${styles.dot} ${i < step ? styles.done : ''} ${i === step ? styles.active : ''}`}
          />
        ))}
        <span className={styles.stepCount}>{step + 1} / {totalSteps}</span>
      </div>

      <div className={styles.card}>
        {!isContactStep ? (
          <QuestionStep
            question={questions[step]}
            value={answers[questions[step].mapping_key] ?? ''}
            onChange={(v) => handleAnswer(questions[step].mapping_key, v)}
          />
        ) : (
          <ContactStep
            email={email} phone={phone} plan={plan}
            onEmail={setEmail} onPhone={setPhone} onPlan={setPlan}
          />
        )}

        {(stepError ?? submitError) && (
          <div className={styles.error}>{stepError ?? submitError}</div>
        )}

        <div className={styles.nav}>
          <button className={styles.btnOutline} onClick={goPrev} disabled={step === 0}>← Назад</button>
          {!isContactStep
            ? <button className={styles.btnPrimary} onClick={goNext}>Далее →</button>
            : <button className={styles.btnPrimary} onClick={handleSubmit} disabled={submitting}>
                {submitting ? 'Создаём заказ...' : 'Оплатить →'}
              </button>
          }
        </div>
      </div>
    </div>
  )
}

function QuestionStep({
  question, value, onChange,
}: { question: Question; value: string; onChange: (v: string) => void }) {
  return (
    <>
      <div className={styles.label}>
        {question.question_text}
        {question.is_required && <span className={styles.req}>*</span>}
      </div>
      {question.ui_type === 'select' ? (
        <div className={styles.options}>
          {question.options.map((opt) => (
            <button
              key={opt.value}
              className={`${styles.optionBtn} ${value === opt.value ? styles.selected : ''}`}
              onClick={() => onChange(opt.value)}
            >
              {opt.label}
            </button>
          ))}
        </div>
      ) : question.ui_type === 'textarea' ? (
        <textarea
          className={styles.input}
          rows={4}
          placeholder="Введите ваш ответ..."
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      ) : (
        <input
          className={styles.input}
          type="text"
          placeholder="Введите ваш ответ..."
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      )}
    </>
  )
}

function ContactStep({
  email, phone, plan, onEmail, onPhone, onPlan,
}: {
  email: string; phone: string; plan: string
  onEmail: (v: string) => void; onPhone: (v: string) => void; onPlan: (v: 'standard' | 'premium') => void
}) {
  return (
    <>
      <div className={styles.label}>Контактные данные и тариф</div>
      <div className={styles.contactForm}>
        <div className={styles.formGroup}>
          <label>Email <span className={styles.req}>*</span></label>
          <input className={styles.input} type="email" placeholder="your@email.com" value={email} onChange={(e) => onEmail(e.target.value)} />
        </div>
        <div className={styles.formGroup}>
          <label>Телефон (необязательно)</label>
          <input className={styles.input} type="tel" placeholder="+7 999 000 00 00" value={phone} onChange={(e) => onPhone(e.target.value)} />
        </div>
        <div className={styles.plansGrid}>
          {(['standard', 'premium'] as const).map((p) => (
            <div
              key={p}
              className={`${styles.planCard} ${plan === p ? styles.planSelected : ''}`}
              onClick={() => onPlan(p)}
            >
              <div className={styles.planName}>{p === 'standard' ? 'Стандарт' : 'Премиум ✨'}</div>
              <div className={styles.planPrice}>{p === 'standard' ? '1 500 ₽' : '2 900 ₽'}</div>
              <div className={styles.planDesc}>{p === 'standard' ? '4 варианта песни' : '8 вариантов + приоритет'}</div>
            </div>
          ))}
        </div>
      </div>
    </>
  )
}
