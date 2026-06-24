import { useEffect, useState } from 'react'
import { adminStatsApi } from '@entities/admin-stats'
import type { DashboardStats } from '@entities/admin-stats'
import { Spinner } from '@shared/ui'
import { A, PageHeader, Panel, ErrorBanner } from '@widgets/admin-layout'

function fmtMoney(kopecks: number) {
  return new Intl.NumberFormat('ru-RU').format(Math.round(kopecks / 100)) + ' ₽'
}

function StatCard({ icon, label, value, hint, accent }: { icon: string; label: string; value: string; hint?: string; accent?: boolean }) {
  return (
    <Panel style={{
      padding: '20px 22px',
      background: accent ? 'linear-gradient(135deg, rgba(0,229,192,0.12), rgba(0,191,165,0.04))' : A.surface,
      border: `1px solid ${accent ? 'rgba(0,229,192,0.28)' : A.border}`,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px' }}>
        <span style={{ fontSize: '16px' }}>{icon}</span>
        <span style={{ fontSize: '11px', fontWeight: 700, color: A.txt3, letterSpacing: '0.06em', textTransform: 'uppercase' }}>{label}</span>
      </div>
      <div style={{ fontSize: '28px', fontWeight: 800, letterSpacing: '-0.03em', color: accent ? A.accent : A.txt, lineHeight: 1 }}>{value}</div>
      {hint && <div style={{ fontSize: '12px', color: A.txt2, marginTop: '8px' }}>{hint}</div>}
    </Panel>
  )
}

function MiniStat({ label, value, tone }: { label: string; value: number; tone: string }) {
  return (
    <div style={{ flex: 1, minWidth: 110, background: A.surface2, border: `1px solid ${A.border}`, borderRadius: '12px', padding: '14px 16px' }}>
      <div style={{ fontSize: '22px', fontWeight: 800, color: tone, letterSpacing: '-0.02em' }}>{value}</div>
      <div style={{ fontSize: '12px', color: A.txt2, marginTop: '4px' }}>{label}</div>
    </div>
  )
}

export function AdminDashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    adminStatsApi.get()
      .then((s) => { setStats(s); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="admin-page admin-page--wide">
      <PageHeader title="Дашборд" subtitle="Сводка по заказам, выручке и ресурсам" />

      {loading && <div style={{ padding: '48px', textAlign: 'center' }}><Spinner /></div>}
      {error && <ErrorBanner>{error}</ErrorBanner>}

      {!loading && stats && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
          {/* Главные метрики */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '14px' }}>
            <StatCard icon="💰" label="Выручка" value={fmtMoney(stats.orders.revenue_kopecks)} hint={`${stats.orders.paid} оплаченных заказов`} accent />
            <StatCard icon="🧾" label="Всего заказов" value={String(stats.orders.total)} hint={`+${stats.orders.today} за 24 часа`} />
            <StatCard icon="✅" label="Готово песен" value={String(stats.orders.completed)} hint={`${stats.orders.processing} в работе`} />
            <StatCard icon="🎚️" label="Аккаунты Suno" value={`${stats.accounts.active}/${stats.accounts.total}`} hint={`🪙 ${stats.accounts.token_balance} токенов`} />
          </div>

          {/* Воронка генерации */}
          <Panel style={{ padding: '22px 24px' }}>
            <div style={{ fontSize: '11px', fontWeight: 700, color: A.txt3, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '14px' }}>Статусы генерации</div>
            <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
              <MiniStat label="Готово" value={stats.orders.completed} tone="#4ade80" />
              <MiniStat label="В работе" value={stats.orders.processing} tone={A.accent} />
              <MiniStat label="Ошибки" value={stats.orders.failed} tone="#f87171" />
            </div>
          </Panel>

          {/* Контент */}
          <Panel style={{ padding: '22px 24px' }}>
            <div style={{ fontSize: '11px', fontWeight: 700, color: A.txt3, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '14px' }}>Контент</div>
            <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
              <MiniStat label="Категорий" value={stats.categories_total} tone={A.txt} />
              <MiniStat label="Примеров активно" value={stats.examples_active} tone={A.txt} />
              <MiniStat label="Примеров всего" value={stats.examples_total} tone={A.txt2} />
            </div>
          </Panel>
        </div>
      )}
    </div>
  )
}
