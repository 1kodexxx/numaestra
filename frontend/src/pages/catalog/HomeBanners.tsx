import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { orderStorage } from '@shared/lib/storage'
import { theme } from '@shared/lib/theme'
import { hasCatalogDraft } from './catalogDraft'

interface HomeBannersProps {
  onResumeDraft: () => void
}

export function HomeBanners({ onResumeDraft }: HomeBannersProps) {
  const orderId = orderStorage.getOrderId()
  const [draftReady, setDraftReady] = useState(false)

  useEffect(() => {
    setDraftReady(hasCatalogDraft())
  }, [])

  if (!orderId && !draftReady) return null

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 16 }}>
      {orderId && (
        <Link
          to={`/status/${orderId}`}
          className="interactive-card"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 12,
            padding: '14px 18px',
            borderRadius: 16,
            background: 'rgba(0,229,192,0.06)',
            border: `1px solid rgba(0,229,192,0.28)`,
            textDecoration: 'none',
            color: '#fff',
          }}
        >
          <div>
            <div style={{ fontSize: 13, fontWeight: 700, color: theme.accent }}>У вас есть активный заказ</div>
            <div style={{ fontSize: 12, color: theme.text2, marginTop: 4 }}>Перейти к статусу и результатам</div>
          </div>
          <span style={{ fontSize: 18, color: theme.accent, flexShrink: 0 }}>→</span>
        </Link>
      )}
      {draftReady && (
        <button
          type="button"
          onClick={() => {
            onResumeDraft()
            setDraftReady(hasCatalogDraft())
          }}
          className="interactive-card chip-press"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 12,
            width: '100%',
            padding: '14px 18px',
            borderRadius: 16,
            background: theme.surface,
            border: `1px solid ${theme.border}`,
            cursor: 'pointer',
            fontFamily: 'inherit',
            color: '#fff',
            textAlign: 'left',
          }}
        >
          <div>
            <div style={{ fontSize: 13, fontWeight: 700 }}>Продолжить черновик песни</div>
            <div style={{ fontSize: 12, color: theme.text2, marginTop: 4 }}>Вы начали заполнять конструктор — закончите и оформите заказ</div>
          </div>
          <span style={{ fontSize: 18, color: theme.accent, flexShrink: 0 }}>→</span>
        </button>
      )}
    </div>
  )
}
