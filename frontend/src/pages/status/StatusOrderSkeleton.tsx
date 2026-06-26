import { theme } from '@shared/lib/theme'

/** Skeleton карточки статуса заказа (вместо голого спиннера). */
export function StatusOrderSkeleton() {
  return (
    <div className="status-order-card status-order-skeleton" aria-hidden>
      <div className="skeleton status-order-skeleton__icon" />
      <div className="skeleton status-order-skeleton__title" />
      <div className="skeleton status-order-skeleton__sub" />
      <div className="status-steps status-order-skeleton__steps">
        {Array.from({ length: 4 }, (_, i) => (
          <div key={i} className="status-step-col">
            <div className="skeleton status-order-skeleton__dot" />
            <div className="skeleton status-order-skeleton__label" />
          </div>
        ))}
      </div>
      <div className="skeleton status-order-skeleton__id" style={{ marginTop: 20 }} />
    </div>
  )
}

export function StatusLookupSkeleton() {
  return (
    <div style={{ maxWidth: 480, margin: '0 auto', padding: '64px 24px' }}>
      <div
        style={{
          background: theme.surface,
          border: `1px solid ${theme.border}`,
          borderRadius: '24px',
          padding: '40px 36px',
          textAlign: 'center',
        }}
      >
        <div className="skeleton" style={{ width: 48, height: 48, borderRadius: 12, margin: '0 auto 16px' }} />
        <div className="skeleton" style={{ height: 24, width: '70%', margin: '0 auto 12px', borderRadius: 8 }} />
        <div className="skeleton" style={{ height: 14, width: '50%', margin: '0 auto 28px', borderRadius: 6 }} />
        <div className="skeleton" style={{ height: 52, borderRadius: 12, marginBottom: 10 }} />
        <div className="skeleton" style={{ height: 52, borderRadius: 12 }} />
      </div>
    </div>
  )
}
