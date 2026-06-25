import { useNavigate, useParams } from 'react-router-dom'
import { Button } from '@shared/ui'
import { useSeo } from '@shared/lib/seo'
import { staggerDelay } from '@shared/lib/motion'
import { findLegalDoc, BUSINESS } from './legalContent'

const TEXT2 = 'rgba(255,255,255,0.55)'
const TEXT3 = 'rgba(255,255,255,0.32)'
const BORDER = 'rgba(255,255,255,0.08)'

export function LegalPage() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const doc = findLegalDoc(slug)

  useSeo({
    title: doc ? doc.title : 'Документ не найден',
    description: doc?.description,
  })

  if (!doc) {
    return (
      <div style={{ maxWidth: 720, margin: '0 auto', padding: '64px 24px', textAlign: 'center' }}>
        <div style={{ fontSize: '40px', marginBottom: '12px' }}>📄</div>
        <div style={{ fontSize: '20px', fontWeight: 800, marginBottom: '16px' }}>Документ не найден</div>
        <Button onClick={() => navigate('/')}>На главную</Button>
      </div>
    )
  }

  return (
    <div style={{ maxWidth: 760, margin: '0 auto', padding: '40px 24px 60px' }} className="fade-in">
      <button
        onClick={() => navigate(-1)}
        style={{ background: 'none', border: 'none', color: TEXT2, fontSize: '13px', cursor: 'pointer', padding: 0, marginBottom: '20px' }}
        onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
        onMouseLeave={(e) => { e.currentTarget.style.color = TEXT2 }}
      >
        ← Назад
      </button>

      <h1 style={{ fontSize: 'clamp(24px, 5vw, 32px)', fontWeight: 800, letterSpacing: '-0.03em', marginBottom: '6px' }}>{doc.title}</h1>
      <div style={{ fontSize: '13px', color: TEXT3, marginBottom: '28px' }}>
        {BUSINESS.brand} · обновлено: {BUSINESS.updated}
      </div>

      {doc.intro && (
        <p style={{ fontSize: '15px', lineHeight: 1.7, color: 'rgba(255,255,255,0.78)', marginBottom: '28px' }}>
          {doc.intro}
        </p>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
        {doc.sections.map((s, i) => (
          <section key={i} className="fade-up" style={staggerDelay(i, 50, 300)}>
            {s.heading && (
              <h2 style={{ fontSize: '17px', fontWeight: 700, letterSpacing: '-0.01em', marginBottom: '10px' }}>{s.heading}</h2>
            )}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
              {s.paragraphs.map((p, j) => (
                <p key={j} style={{ fontSize: '14px', lineHeight: 1.7, color: 'rgba(255,255,255,0.72)' }}>{p}</p>
              ))}
            </div>
          </section>
        ))}
      </div>

      <div style={{ borderTop: `1px solid ${BORDER}`, marginTop: '36px', paddingTop: '20px', fontSize: '12px', color: TEXT3 }}>
        Остались вопросы? Напишите нам: {BUSINESS.email}
      </div>
    </div>
  )
}
