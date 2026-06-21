import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCatalog } from '@features/load-catalog'
import { EXAMPLE_SONGS } from '@shared/data/examples'
import type { Category } from '@entities/category'

/* ─── tokens ───────────────────────────────────────────────── */
const S = {
  panel:      { width: 220, bg: '#080808', border: 'rgba(255,255,255,0.06)' },
  card:       { bg: '#111111', hover: '#161616', radius: 12 },
  accent:     '#00e5c0',
  accentBg:   'rgba(0,229,192,0.10)',
  text:       '#ffffff',
  text2:      'rgba(255,255,255,0.48)',
  text3:      'rgba(255,255,255,0.22)',
  border:     'rgba(255,255,255,0.06)',
} as const

/* ─── side-panel list item ──────────────────────────────────── */
function SideItem({ title, sub, onClick }: { title: string; sub?: string; onClick: () => void }) {
  const [hovered, setHovered] = useState(false)
  return (
    <button
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        width: '100%',
        display: 'block',
        textAlign: 'left',
        background: hovered ? S.card.hover : 'transparent',
        border: 'none',
        borderRadius: '10px',
        padding: '11px 14px',
        marginBottom: '2px',
        cursor: 'pointer',
        transition: 'background 0.15s',
        position: 'relative',
      }}
    >
      {hovered && (
        <span style={{
          position: 'absolute', left: 0, top: '20%', height: '60%',
          width: '2px', borderRadius: '1px', background: S.accent,
        }} />
      )}
      <div style={{ fontSize: '13px', fontWeight: 600, color: hovered ? S.text : S.text2, transition: 'color 0.15s', lineHeight: 1.3 }}>
        {title}
      </div>
      {sub && (
        <div style={{ fontSize: '11px', color: S.text3, marginTop: '2px', fontWeight: 500 }}>{sub}</div>
      )}
    </button>
  )
}

/* ─── search input ──────────────────────────────────────────── */
function SearchInput({ onClick }: { onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '12px',
        width: '100%',
        maxWidth: '560px',
        background: 'rgba(255,255,255,0.04)',
        border: '1px solid rgba(255,255,255,0.09)',
        borderRadius: '16px',
        padding: '16px 22px',
        cursor: 'text',
        transition: 'all 0.2s',
        textAlign: 'left',
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.borderColor = 'rgba(0,229,192,0.3)'
        e.currentTarget.style.background = 'rgba(255,255,255,0.05)'
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.borderColor = 'rgba(255,255,255,0.09)'
        e.currentTarget.style.background = 'rgba(255,255,255,0.04)'
      }}
    >
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="rgba(255,255,255,0.3)" strokeWidth="2" strokeLinecap="round">
        <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
      </svg>
      <span style={{ fontSize: '15px', color: 'rgba(255,255,255,0.28)', fontWeight: 400 }}>
        Опишите вашу песню...
      </span>
    </button>
  )
}

/* ─── category grid card ─────────────────────────────────────── */
function CategoryCard({ cat, onClick }: { cat: Category; onClick: () => void }) {
  const [hovered, setHovered] = useState(false)
  return (
    <button
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        aspectRatio: '1 / 1',
        background: hovered
          ? 'linear-gradient(135deg, #00f0cc 0%, #00c9aa 100%)'
          : 'linear-gradient(135deg, #00e5c0 0%, #00bfa5 100%)',
        border: 'none',
        borderRadius: '20px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        textAlign: 'center',
        padding: '16px',
        cursor: 'pointer',
        transform: hovered ? 'scale(1.035)' : 'scale(1)',
        boxShadow: hovered ? '0 8px 32px rgba(0,229,192,0.25)' : '0 2px 8px rgba(0,0,0,0.4)',
        transition: 'all 0.2s cubic-bezier(0.34,1.56,0.64,1)',
      }}
    >
      <span style={{ fontSize: '14px', fontWeight: 700, color: '#0a0a0a', lineHeight: 1.25 }}>
        {cat.title}
      </span>
    </button>
  )
}

/* ─── loading spinner ────────────────────────────────────────── */
function Spinner() {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 'calc(100vh - 60px)' }}>
      <div
        className="spin-anim"
        style={{
          width: 36, height: 36,
          borderRadius: '50%',
          border: '2px solid rgba(255,255,255,0.08)',
          borderTopColor: '#00e5c0',
        }}
      />
    </div>
  )
}

/* ─── main page ──────────────────────────────────────────────── */
export function CatalogPage() {
  const { categories, loading } = useCatalog()
  const navigate = useNavigate()
  const [briefOpen, setBriefOpen] = useState(false)
  const [brief, setBrief] = useState('')

  if (loading) return <Spinner />

  const topCats = categories.slice(0, Math.min(8, categories.length))

  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 60px)', overflow: 'hidden' }}>

      {/* ── Left panel: examples ── */}
      <aside style={{
        width: S.panel.width,
        borderRight: `1px solid ${S.panel.border}`,
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}>
        <div style={{ flex: 1, overflowY: 'auto', padding: '20px 12px 12px' }}>
          {EXAMPLE_SONGS.map((ex) => (
            <SideItem
              key={ex.id}
              title={ex.title}
              sub={ex.category}
              onClick={() => navigate(`/examples/${ex.id}`)}
            />
          ))}
        </div>
        <div style={{ padding: '12px 16px 16px', borderTop: `1px solid ${S.panel.border}` }}>
          <span style={{ fontSize: '11px', fontWeight: 600, color: S.text3, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
            Примеры работ
          </span>
        </div>
      </aside>

      {/* ── Center ── */}
      <main style={{ flex: 1, display: 'flex', flexDirection: 'column', overflowY: 'auto' }}>

        {/* Search / brief area */}
        <div style={{ padding: '28px 32px 20px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '12px' }}>
          {briefOpen ? (
            <div style={{ width: '100%', maxWidth: '560px' }} className="fade-in">
              <button
                onClick={() => setBriefOpen(false)}
                style={{
                  background: 'none', border: 'none', color: 'rgba(255,255,255,0.35)',
                  fontSize: '13px', cursor: 'pointer', marginBottom: '10px',
                  display: 'flex', alignItems: 'center', gap: '6px', padding: 0,
                  transition: 'color 0.15s',
                }}
                onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
                onMouseLeave={(e) => { e.currentTarget.style.color = 'rgba(255,255,255,0.35)' }}
              >
                ← К категориям
              </button>
              <textarea
                autoFocus
                placeholder="Опишите вашу идею: кому, по какому поводу, какое настроение..."
                value={brief}
                onChange={(e) => setBrief(e.target.value)}
                rows={5}
                style={{
                  width: '100%',
                  background: 'rgba(255,255,255,0.03)',
                  border: '1px solid rgba(0,229,192,0.4)',
                  borderRadius: '16px',
                  padding: '20px 22px',
                  color: '#fff',
                  fontSize: '15px',
                  resize: 'none',
                  outline: 'none',
                  lineHeight: 1.65,
                  fontFamily: 'inherit',
                  boxShadow: '0 0 0 4px rgba(0,229,192,0.06)',
                }}
              />
            </div>
          ) : (
            <SearchInput onClick={() => setBriefOpen(true)} />
          )}

          <p style={{ fontSize: '12px', color: S.text3, fontWeight: 500 }}>
            или выберите категорию ниже
          </p>
        </div>

        {/* Category grid */}
        <div style={{ padding: '0 32px 32px' }}>
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(4, 1fr)',
            gap: '12px',
          }}>
            {categories.map((cat) => (
              <CategoryCard
                key={cat.id}
                cat={cat}
                onClick={() => navigate(`/category/${cat.id}`)}
              />
            ))}
          </div>
        </div>
      </main>

      {/* ── Right panel: categories ── */}
      <aside style={{
        width: S.panel.width,
        borderLeft: `1px solid ${S.panel.border}`,
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}>
        <div style={{ flex: 1, overflowY: 'auto', padding: '20px 12px 12px' }}>
          {topCats.map((cat, i) => (
            <SideItem
              key={cat.id}
              title={cat.title}
              sub={`#${i + 1}`}
              onClick={() => navigate(`/category/${cat.id}`)}
            />
          ))}
        </div>
        <div style={{ padding: '12px 16px 16px', borderTop: `1px solid ${S.panel.border}` }}>
          <span style={{ fontSize: '11px', fontWeight: 600, color: S.text3, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
            Топ категорий
          </span>
        </div>
      </aside>

    </div>
  )
}
