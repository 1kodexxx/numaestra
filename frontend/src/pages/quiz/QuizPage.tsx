import { useEffect, useState } from 'react'
import { useNavigate, useParams, useLocation } from 'react-router-dom'
import { categoryApi } from '@entities/category'
import { useCreateOrder } from '@features/create-order'
import { Spinner } from '@shared/ui'
import type { Question, WizardData } from '@entities/category'

const inputCls = [
  'w-full bg-bg3 border border-border rounded-lg px-4 py-3',
  'text-txt text-[15px] outline-none transition-colors',
  'focus:border-accent2 placeholder:text-muted font-[inherit] resize-y',
].join(' ')

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
    categoryApi
      .wizard(categoryId)
      .then(setWizard)
      .catch((err: Error) => setLoadError(err.message))
  }, [categoryId])

  if (loadError) return (
    <div className="bg-error/10 border border-error/30 rounded-lg px-4 py-3 text-error text-sm m-4">
      {loadError}
    </div>
  )
  if (!wizard) return <div className="text-center py-10"><Spinner /></div>

  const questions: Question[] = wizard.questions
  const totalSteps = questions.length + 1
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
    const brief =
      questions
        .map((q) => answers[q.mapping_key] ? `${q.question_text}: ${answers[q.mapping_key]}` : null)
        .filter(Boolean)
        .join('; ') || 'Персональная песня'
    await submit({ email, phone, brief, category_id: categoryId, answers })
  }

  return (
    <div className="max-w-2xl mx-auto px-6 pt-10 pb-20">
      <div className="mb-8">
        <button
          className="text-muted bg-transparent border-none cursor-pointer text-sm inline-flex items-center gap-1.5 mb-4 hover:text-txt transition-colors"
          onClick={() => navigate('/')}
        >
          ← Назад
        </button>
        <div className="text-[28px] font-extrabold">{categoryTitle}</div>
        <div className="text-muted mt-1.5">Ответьте на несколько вопросов</div>
      </div>

      <div className="flex items-center gap-1 mb-8">
        {Array.from({ length: totalSteps }, (_, i) => (
          <div
            key={i}
            className={`w-2 h-2 rounded-full transition-colors ${
              i < step ? 'bg-accent2' : i === step ? 'bg-accent' : 'bg-border'
            }`}
          />
        ))}
        <span className="text-[13px] text-muted ml-2.5">{step + 1} / {totalSteps}</span>
      </div>

      <div className="bg-bg2 border border-border rounded-xl p-7">
        {!isContactStep ? (
          <QuestionStep
            question={questions[step]}
            value={answers[questions[step].mapping_key] ?? ''}
            onChange={(v) => handleAnswer(questions[step].mapping_key, v)}
          />
        ) : (
          <ContactStep
            email={email} phone={phone}
            onEmail={setEmail} onPhone={setPhone}
          />
        )}

        {(stepError ?? submitError) && (
          <div className="bg-error/10 border border-error/30 rounded-lg px-4 py-3 text-error text-sm my-3">
            {stepError ?? submitError}
          </div>
        )}

        <div className="flex justify-between mt-7 gap-3">
          <button
            className="px-6 py-2.5 rounded-lg text-sm font-semibold bg-transparent border border-accent2 text-accent cursor-pointer hover:bg-accent/10 disabled:opacity-40 disabled:cursor-not-allowed transition-all"
            onClick={goPrev}
            disabled={step === 0}
          >
            ← Назад
          </button>
          {!isContactStep
            ? (
              <button
                className="px-6 py-2.5 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] transition-all"
                onClick={goNext}
              >
                Далее →
              </button>
            ) : (
              <button
                className="px-6 py-2.5 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] disabled:opacity-60 disabled:cursor-not-allowed transition-all"
                onClick={handleSubmit}
                disabled={submitting}
              >
                {submitting ? 'Создаём заказ...' : 'Оплатить →'}
              </button>
            )
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
      <div className="text-lg font-semibold mb-5">
        {question.question_text}
        {question.is_required && <span className="text-error ml-1">*</span>}
      </div>
      {/* select/tags/radio — все три типа рендерятся как кнопки-варианты */}
      {(question.ui_type === 'select' || question.ui_type === 'tags' || question.ui_type === 'radio') ? (
        <div className={`flex gap-2.5 ${question.ui_type === 'radio' ? 'flex-col' : 'flex-wrap'}`}>
          {question.options.map((opt) => (
            <button
              key={opt.value}
              className={`border rounded-lg px-4 py-3 text-left text-txt cursor-pointer text-[15px] transition-all ${
                question.ui_type === 'radio' ? 'w-full bg-bg3' : 'bg-bg3'
              } ${
                value === opt.value
                  ? 'border-accent bg-accent/15 text-accent'
                  : 'border-border hover:border-accent2 hover:bg-accent2/10'
              }`}
              onClick={() => onChange(opt.value)}
            >
              {opt.label}
            </button>
          ))}
        </div>
      ) : question.ui_type === 'textarea' ? (
        <textarea
          className={inputCls}
          rows={4}
          placeholder="Введите ваш ответ..."
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      ) : (
        <input
          className={inputCls}
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
  email, phone, onEmail, onPhone,
}: {
  email: string; phone: string
  onEmail: (v: string) => void; onPhone: (v: string) => void
}) {
  return (
    <>
      <div className="text-lg font-semibold mb-5">Контактные данные</div>
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5 text-sm text-muted">
          <label>Email <span className="text-error">*</span></label>
          <input className={inputCls} type="email" placeholder="your@email.com" value={email} onChange={(e) => onEmail(e.target.value)} />
        </div>
        <div className="flex flex-col gap-1.5 text-sm text-muted">
          <label>Телефон (необязательно)</label>
          <input className={inputCls} type="tel" placeholder="+7 999 000 00 00" value={phone} onChange={(e) => onPhone(e.target.value)} />
        </div>
        <div className="rounded-[10px] p-4 text-center border-2 border-accent bg-accent/12">
          <div className="font-bold text-base">4 версии песни</div>
          <div className="text-gold text-[22px] font-extrabold my-1.5">2 000 ₽</div>
          <div className="text-xs text-muted">Один платёж, без подписок и тарифов</div>
        </div>
      </div>
    </>
  )
}
