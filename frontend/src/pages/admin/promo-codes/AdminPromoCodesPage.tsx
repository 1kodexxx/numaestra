import { useEffect, useState } from 'react'
import { adminPromoApi } from '@entities/admin-promo'
import type { PromoCode, CreatePromoPayload } from '@entities/admin-promo'
import { Spinner, Button } from '@shared/ui'
import { ApiError } from '@shared/api'
import { A, PageHeader, Panel, ErrorBanner, EmptyState, Field, Select } from '@widgets/admin-layout'

const inputStyle: React.CSSProperties = {
  width: '100%', boxSizing: 'border-box',
  background: 'rgba(255,255,255,0.05)', border: '1px solid rgba(255,255,255,0.12)',
  borderRadius: 8, padding: '8px 12px', color: '#fff', fontSize: 14,
  outline: 'none',
}


function formatDiscount(p: PromoCode): string {
  if (p.discount_type === 'percent') return `${p.discount_value}%`
  return `${p.discount_value} ₽`
}

function formatDate(iso?: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('ru-RU')
}

export function AdminPromoCodesPage() {
  const [promos, setPromos] = useState<PromoCode[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  // form fields
  const [code, setCode] = useState('')
  const [discountType, setDiscountType] = useState<'percent' | 'fixed_rub'>('percent')
  const [discountValue, setDiscountValue] = useState('')
  const [maxUses, setMaxUses] = useState('')
  const [validUntil, setValidUntil] = useState('')
  const [description, setDescription] = useState('')

  function load() {
    setLoading(true)
    adminPromoApi
      .list()
      .then(({ items, total }) => { setPromos(items); setTotal(total); setError(null) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  function resetForm() {
    setCode(''); setDiscountType('percent'); setDiscountValue('')
    setMaxUses(''); setValidUntil(''); setDescription('')
    setFormError(null)
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    const val = parseInt(discountValue, 10)
    if (!code.trim() || isNaN(val) || val <= 0) {
      setFormError('Укажите код и корректное значение скидки')
      return
    }
    setFormError(null); setSubmitting(true)
    try {
      const payload: CreatePromoPayload = {
        code: code.trim().toUpperCase(),
        discount_type: discountType,
        discount_value: val,
      }
      if (maxUses) payload.max_uses = parseInt(maxUses, 10)
      if (validUntil) payload.valid_until = new Date(validUntil).toISOString()
      if (description.trim()) payload.description = description.trim()

      await adminPromoApi.create(payload)
      resetForm(); setShowForm(false); load()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Ошибка создания')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleToggle(promo: PromoCode) {
    try {
      await adminPromoApi.update(promo.id, {
        description: promo.description,
        is_active: !promo.is_active,
        max_uses: promo.max_uses ?? undefined,
        valid_until: promo.valid_until,
      })
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка обновления')
    }
  }

  async function handleDelete(id: string) {
    if (!confirm('Удалить промокод?')) return
    try {
      await adminPromoApi.delete(id)
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка удаления')
    }
  }

  return (
    <div style={{ maxWidth: 900 }}>
      <PageHeader
        title={`Промокоды (${total})`}
        action={<Button size="sm" onClick={() => { resetForm(); setShowForm(v => !v) }}>
          {showForm ? 'Отмена' : '+ Создать'}
        </Button>}
      />

      {showForm && (
        <Panel style={{ marginBottom: 20 }}>
          <form onSubmit={handleCreate}>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <Field label="Код промокода">
                <input style={inputStyle} value={code} onChange={e => setCode(e.target.value.toUpperCase())} placeholder="SUMMER20" />
              </Field>
              <Field label="Тип скидки">
                <Select value={discountType} onChange={v => setDiscountType(v as 'percent' | 'fixed_rub')}>
                  <option value="percent">Процент (%)</option>
                  <option value="fixed_rub">Фиксированная сумма (₽)</option>
                </Select>
              </Field>
              <Field label={discountType === 'percent' ? 'Скидка (%)' : 'Скидка (₽)'}>
                <input style={inputStyle} value={discountValue} onChange={e => setDiscountValue(e.target.value)} placeholder={discountType === 'percent' ? '20' : '500'} type="number" min="1" />
              </Field>
              <Field label="Лимит использований (пусто = без лимита)">
                <input style={inputStyle} value={maxUses} onChange={e => setMaxUses(e.target.value)} placeholder="100" type="number" min="1" />
              </Field>
              <Field label="Действует до (пусто = бессрочно)">
                <input style={inputStyle} value={validUntil} onChange={e => setValidUntil(e.target.value)} type="date" />
              </Field>
              <Field label="Описание (для себя)">
                <input style={inputStyle} value={description} onChange={e => setDescription(e.target.value)} placeholder="Летняя акция 2025" />
              </Field>
            </div>
            {formError && <ErrorBanner>{formError}</ErrorBanner>}
            <div style={{ marginTop: 14 }}>
              <Button type="submit" size="sm" disabled={submitting}>
                {submitting ? 'Создание…' : 'Создать промокод'}
              </Button>
            </div>
          </form>
        </Panel>
      )}

      {error && <ErrorBanner>{error}</ErrorBanner>}

      {loading ? (
        <Spinner />
      ) : promos.length === 0 ? (
        <EmptyState icon="🎟️" text="Промокодов пока нет" />
      ) : (
        <Panel>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.08)', textAlign: 'left' }}>
                {['Код', 'Скидка', 'Использований', 'Действует до', 'Статус', ''].map(h => (
                  <th key={h} style={{ padding: '8px 10px', fontWeight: 600, color: 'rgba(255,255,255,0.5)' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {promos.map(p => (
                <tr key={p.id} style={{ borderBottom: '1px solid rgba(255,255,255,0.04)' }}>
                  <td style={{ padding: '10px' }}>
                    <code style={{ fontWeight: 700 }}>{p.code}</code>
                    {p.description && <div style={{ fontSize: 11, color: 'rgba(255,255,255,0.4)', marginTop: 2 }}>{p.description}</div>}
                  </td>
                  <td style={{ padding: '10px' }}>{formatDiscount(p)}</td>
                  <td style={{ padding: '10px' }}>
                    {p.current_uses}{p.max_uses != null ? ` / ${p.max_uses}` : ''}
                  </td>
                  <td style={{ padding: '10px' }}>{formatDate(p.valid_until)}</td>
                  <td style={{ padding: '10px' }}>
                    <span style={{ color: p.is_active ? '#00e5c0' : 'rgba(255,255,255,0.3)', fontWeight: 600 }}>
                      {p.is_active ? 'Активен' : 'Отключён'}
                    </span>
                  </td>
                  <td style={{ padding: '10px', whiteSpace: 'nowrap' }}>
                    <A onClick={() => handleToggle(p)} style={{ marginRight: 12 }}>
                      {p.is_active ? 'Отключить' : 'Включить'}
                    </A>
                    <A onClick={() => handleDelete(p.id)} style={{ color: '#f87171' }}>Удалить</A>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Panel>
      )}
    </div>
  )
}
