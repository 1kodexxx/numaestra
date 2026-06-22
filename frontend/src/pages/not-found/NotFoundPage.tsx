import { useNavigate } from 'react-router-dom'
import { Button } from '@shared/ui'
import { useSeo } from '@shared/lib/seo'

const TEXT2 = 'rgba(255,255,255,0.55)'

export function NotFoundPage() {
  const navigate = useNavigate()
  useSeo({ title: 'Страница не найдена', description: 'Запрошенная страница не существует.', noindex: true })

  return (
    <div style={{
      minHeight: 'calc(100dvh - 60px)', display: 'flex', flexDirection: 'column',
      alignItems: 'center', justifyContent: 'center', gap: '8px', padding: 'clamp(24px, 6vh, 48px) 24px', textAlign: 'center',
    }}>
      <div className="gradient-text" style={{ fontSize: 'clamp(64px, 18vw, 96px)', fontWeight: 800, letterSpacing: '-0.04em', lineHeight: 1 }}>
        404
      </div>
      <div style={{ fontSize: 'clamp(18px, 4vw, 22px)', fontWeight: 800, letterSpacing: '-0.02em', marginTop: '8px' }}>
        Такой страницы нет
      </div>
      <div style={{ fontSize: '14px', color: TEXT2, maxWidth: '360px', lineHeight: 1.6, marginBottom: '20px' }}>
        Возможно, ссылка устарела или содержит опечатку. Вернитесь на главную и соберите свою песню.
      </div>
      <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap', justifyContent: 'center' }}>
        <Button size="lg" onClick={() => navigate('/')}>На главную</Button>
        <Button size="lg" variant="outlined" onClick={() => navigate('/status')}>Мой заказ</Button>
      </div>
    </div>
  )
}
