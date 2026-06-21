import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCatalog } from '@features/load-catalog'
import { EXAMPLE_SONGS } from '@shared/data/examples'
import { Button, useRipple } from '@shared/ui'
import type { Category } from '@entities/category'

/* ─── tokens ─── */
const ACCENT  = '#00e5c0'
const BORDER  = 'rgba(255,255,255,0.07)'
const TEXT2   = 'rgba(255,255,255,0.48)'
const TEXT3   = 'rgba(255,255,255,0.2)'

/* ─── breakpoints ─── */
function useBreakpoint() {
  const [w, setW] = useState(typeof window !== 'undefined' ? window.innerWidth : 1200)
  useEffect(() => {
    const fn = () => setW(window.innerWidth)
    window.addEventListener('resize', fn)
    return () => { window.removeEventListener('resize', fn) }
  }, [])
  return { isMobile: w < 640, isTablet: w >= 640 && w < 1024, isDesktop: w >= 1024 }
}

/* ─── icon map: matches Russian and English keywords ─── */
const ICON_RULES: [string, string][] = [
  ['свадьб', '💍'], ['wedding', '💍'],
  ['день рожден', '🎂'], ['birthday', '🎂'],
  ['корпоратив', '🏢'], ['corporate', '🏢'],
  ['юбилей', '🥂'], ['anniversary', '🥂'],
  ['любов', '❤️'], ['love', '❤️'],
  ['детск', '🎈'], ['kids', '🎈'],
  ['выпускн', '🎓'], ['graduation', '🎓'],
  ['новый год', '🎆'], ['newyear', '🎆'],
  ['путешеств', '✈️'], ['travel', '✈️'],
  ['спорт', '🏆'], ['sport', '🏆'],
  ['дружб', '🤝'], ['friendship', '🤝'],
  ['романтик', '🌹'], ['romantic', '🌹'],
  ['мама', '🌸'], ['маме', '🌸'],
  ['папа', '👔'], ['папе', '👔'],
  ['новорожден', '👶'], ['baby', '👶'],
  ['развод', '💔'], ['breakup', '💔'],
  ['мотивац', '🔥'], ['roast', '🔥'],
]

const FALLBACK_ICONS = ['🎵','🎸','🎹','🎺','🥁','🎷','🎻','🪗','🎤','🪘','🎙️','🎚️']

function getIcon(cat: Category, idx: number): string {
  const haystack = `${cat.id} ${cat.title}`.toLowerCase()
  for (const [key, icon] of ICON_RULES) {
    if (haystack.includes(key)) return icon
  }
  return FALLBACK_ICONS[idx % FALLBACK_ICONS.length]
}

/* ─── side panel item ─── */
function SideItem({ title, sub, onClick }: { title: string; sub?: string; onClick: () => void }) {
  const [h, setH] = useState(false)
  return (
    <button
      onClick={onClick}
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
      style={{
        width: '100%', display: 'block', textAlign: 'left',
        background: h ? 'rgba(255,255,255,0.04)' : 'transparent',
        border: 'none', borderRadius: '10px',
        padding: '10px 12px', marginBottom: '2px',
        cursor: 'pointer', transition: 'background 0.15s',
        position: 'relative',
      }}
    >
      {h && (
        <span style={{
          position: 'absolute', left: 0, top: '18%', height: '64%',
          width: '2px', borderRadius: '1px', background: ACCENT,
        }} />
      )}
      <div style={{
        fontSize: '13px', fontWeight: 600, lineHeight: 1.3,
        color: h ? '#fff' : TEXT2,
        paddingLeft: h ? '6px' : '0',
        transition: 'all 0.15s',
      }}>
        {title}
      </div>
      {sub && (
        <div style={{
          fontSize: '11px', color: TEXT3, marginTop: '2px',
          paddingLeft: h ? '6px' : '0', transition: 'padding 0.15s',
        }}>
          {sub}
        </div>
      )}
    </button>
  )
}

/* ─── hero ─── */
function Hero({ compact }: { compact: boolean }) {
  return (
    <div style={{ textAlign: 'center', maxWidth: '600px', margin: '0 auto' }}>
      <div style={{
        display: 'inline-flex', alignItems: 'center', gap: '7px',
        background: 'rgba(0,229,192,0.08)', border: '1px solid rgba(0,229,192,0.18)',
        borderRadius: '20px', padding: '5px 13px 5px 11px', marginBottom: compact ? '14px' : '18px',
      }}>
        <span style={{ fontSize: '12px' }}>🎙️</span>
        <span style={{ fontSize: '11px', fontWeight: 700, color: ACCENT, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
          AI-студия персональных песен
        </span>
      </div>

      <h1 style={{
        fontSize: compact ? '28px' : '40px',
        fontWeight: 800, letterSpacing: '-0.035em', lineHeight: 1.1,
        marginBottom: '14px',
      }}>
        Песня, написанная{' '}
        <span style={{
          background: 'linear-gradient(120deg, #00e5c0, #00bfa5)',
          WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent', backgroundClip: 'text',
        }}>
          лично для вас
        </span>
      </h1>

      <p style={{
        fontSize: compact ? '14px' : '16px', color: TEXT2, lineHeight: 1.6,
        maxWidth: '460px', margin: '0 auto',
      }}>
        Опишите повод — и получите 4 готовые версии трека уже через 24 часа
      </p>

      {/* trust pills */}
      <div style={{
        display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: '8px',
        marginTop: compact ? '16px' : '22px',
      }}>
        {['4 версии трека', 'Готово за 24 часа', '2 000 ₽ · без подписок'].map(t => (
          <div key={t} style={{
            display: 'inline-flex', alignItems: 'center', gap: '6px',
            background: 'rgba(255,255,255,0.03)', border: `1px solid ${BORDER}`,
            borderRadius: '20px', padding: '6px 13px',
            fontSize: '12px', fontWeight: 500, color: 'rgba(255,255,255,0.6)',
          }}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke={ACCENT} strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
              <path d="M20 6 9 17l-5-5"/>
            </svg>
            {t}
          </div>
        ))}
      </div>
    </div>
  )
}

/* ─── search bar ─── */
function SearchBar({ onClick }: { onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      style={{
        display: 'flex', alignItems: 'center', gap: '12px',
        width: '100%', maxWidth: '580px',
        background: 'rgba(255,255,255,0.03)',
        border: '1px solid rgba(255,255,255,0.08)',
        borderRadius: '14px', padding: '15px 20px',
        cursor: 'text', transition: 'all 0.2s', textAlign: 'left',
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.borderColor = 'rgba(0,229,192,0.35)'
        e.currentTarget.style.background = 'rgba(255,255,255,0.045)'
        e.currentTarget.style.boxShadow = '0 0 0 4px rgba(0,229,192,0.05)'
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.borderColor = 'rgba(255,255,255,0.08)'
        e.currentTarget.style.background = 'rgba(255,255,255,0.03)'
        e.currentTarget.style.boxShadow = 'none'
      }}
    >
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none"
        stroke="rgba(255,255,255,0.25)" strokeWidth="2.5" strokeLinecap="round">
        <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
      </svg>
      <span style={{ fontSize: '14px', color: 'rgba(255,255,255,0.25)', fontWeight: 400, letterSpacing: '0.01em' }}>
        Опишите вашу песню или выберите категорию...
      </span>
    </button>
  )
}

/* ─── category card ─── */
function CategoryCard({ cat, index, onClick }: { cat: Category; index: number; onClick: () => void }) {
  const [h, setH] = useState(false)
  const icon = getIcon(cat, index)
  const { onPointerDown, rippleEl } = useRipple('rgba(0,0,0,0.35)')

  return (
    <button
      onClick={onClick}
      onPointerDown={onPointerDown}
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
      aria-label={`Категория: ${cat.title}`}
      style={{
        aspectRatio: '1 / 1',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: '12px',
        padding: '18px 14px',
        background: h
          ? 'linear-gradient(145deg, #05f0cb 0%, #00cdb0 100%)'
          : 'linear-gradient(145deg, #00e5c0 0%, #00c4a7 100%)',
        border: 'none',
        borderRadius: '22px',
        cursor: 'pointer',
        transform: h ? 'translateY(-4px) scale(1.02)' : 'translateY(0) scale(1)',
        boxShadow: h
          ? '0 18px 44px rgba(0,229,192,0.3), 0 4px 12px rgba(0,0,0,0.3)'
          : '0 2px 12px rgba(0,0,0,0.35)',
        transition: 'all 0.24s cubic-bezier(0.34, 1.4, 0.64, 1)',
        overflow: 'hidden',
        position: 'relative',
      }}
    >
      {/* inner highlight */}
      <div style={{
        position: 'absolute', top: 0, left: 0, right: 0, height: '55%',
        background: 'linear-gradient(180deg, rgba(255,255,255,0.2) 0%, transparent 100%)',
        borderRadius: '22px 22px 0 0', pointerEvents: 'none',
      }} />

      {/* icon badge */}
      <div style={{
        width: '52px', height: '52px', borderRadius: '50%',
        background: h ? 'rgba(255,255,255,0.32)' : 'rgba(255,255,255,0.22)',
        border: '1px solid rgba(255,255,255,0.3)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        transition: 'all 0.24s', position: 'relative',
        transform: h ? 'scale(1.08)' : 'scale(1)',
      }}>
        <span style={{ fontSize: '26px', lineHeight: 1 }}>{icon}</span>
      </div>

      <span style={{
        fontSize: '13px', fontWeight: 700, color: '#072420',
        lineHeight: 1.25, letterSpacing: '-0.01em', textAlign: 'center',
        position: 'relative',
      }}>
        {cat.title}
      </span>
      {rippleEl}
    </button>
  )
}

/* ─── skeleton card ─── */
function SkeletonCard() {
  return (
    <div className="skeleton" style={{ aspectRatio: '1 / 1', borderRadius: '22px' }} />
  )
}

/* ─── mobile category chips ─── */
function MobileCategoryRow({ cats }: { cats: Category[] }) {
  const navigate = useNavigate()
  return (
    <div style={{
      display: 'flex', gap: '8px', overflowX: 'auto', padding: '0 16px 4px',
      scrollbarWidth: 'none', WebkitOverflowScrolling: 'touch',
    } as React.CSSProperties}>
      {cats.map(cat => (
        <button
          key={cat.id}
          onClick={() => navigate(`/category/${cat.id}`)}
          style={{
            flexShrink: 0, padding: '7px 14px',
            background: 'rgba(0,229,192,0.08)',
            border: '1px solid rgba(0,229,192,0.2)',
            borderRadius: '20px', color: ACCENT,
            fontSize: '13px', fontWeight: 600,
            cursor: 'pointer', whiteSpace: 'nowrap',
          }}
        >
          {cat.title}
        </button>
      ))}
    </div>
  )
}

/* ─── main ─── */
export function CatalogPage() {
  const { categories, loading } = useCatalog()
  const navigate = useNavigate()
  const { isMobile, isTablet, isDesktop } = useBreakpoint()
  const [briefOpen, setBriefOpen] = useState(false)
  const [brief, setBrief] = useState('')

  const PANEL_W = 210
  const gridCols = isMobile ? 'repeat(2, 1fr)' : isTablet ? 'repeat(3, 1fr)' : 'repeat(4, 1fr)'
  const gridPad = isMobile ? '4px 16px 32px' : isTablet ? '4px 28px 40px' : '4px 40px 40px'
  const heroPad = isMobile ? '24px 16px 8px' : isTablet ? '32px 28px 10px' : '40px 40px 12px'
  const searchPad = isMobile ? '16px 16px 12px' : '20px 40px 18px'

  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 60px)', overflow: 'hidden' }}>

      {/* ── Left panel: desktop only ── */}
      {isDesktop && (
        <aside style={{
          width: PANEL_W, flexShrink: 0,
          borderRight: `1px solid ${BORDER}`,
          display: 'flex', flexDirection: 'column', overflow: 'hidden',
        }}>
          <div style={{ flex: 1, overflowY: 'auto', padding: '18px 10px 10px' }}>
            <div style={{ fontSize: '10px', fontWeight: 700, color: TEXT3, letterSpacing: '0.08em', textTransform: 'uppercase', padding: '0 12px 10px' }}>
              Послушать примеры
            </div>
            {EXAMPLE_SONGS.map(ex => (
              <SideItem key={ex.id} title={ex.title} sub={ex.category} onClick={() => navigate(`/examples/${ex.id}`)} />
            ))}
          </div>
        </aside>
      )}

      {/* ── Center ── */}
      <main style={{ flex: 1, display: 'flex', flexDirection: 'column', overflowY: 'auto', minWidth: 0 }}>

        {/* Hero */}
        <div style={{ padding: heroPad }}>
          <Hero compact={isMobile} />
        </div>

        {/* Mobile category chips */}
        {!isDesktop && !loading && (
          <div style={{ paddingBottom: '4px' }}>
            <div style={{ padding: `0 ${isMobile ? 16 : 28}px 8px` }}>
              <span style={{ fontSize: '11px', fontWeight: 700, color: TEXT3, letterSpacing: '0.07em', textTransform: 'uppercase' }}>
                Категории
              </span>
            </div>
            <MobileCategoryRow cats={categories.slice(0, 10)} />
          </div>
        )}

        {/* Search */}
        <div style={{ padding: searchPad, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '10px' }}>
          {briefOpen ? (
            <div style={{ width: '100%', maxWidth: '580px' }} className="fade-in">
              <button
                onClick={() => setBriefOpen(false)}
                style={{ background: 'none', border: 'none', color: TEXT2, fontSize: '13px', cursor: 'pointer', marginBottom: '10px', padding: 0 }}
                onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
                onMouseLeave={(e) => { e.currentTarget.style.color = TEXT2 }}
              >
                ← К категориям
              </button>
              <textarea
                autoFocus
                placeholder="Расскажите: кому, по какому поводу, какое настроение, жанр..."
                value={brief}
                onChange={(e) => setBrief(e.target.value)}
                rows={5}
                style={{
                  width: '100%',
                  background: 'rgba(255,255,255,0.03)',
                  border: '1px solid rgba(0,229,192,0.4)',
                  borderRadius: '14px', padding: '18px 20px',
                  color: '#fff', fontSize: '14px', resize: 'none', outline: 'none',
                  lineHeight: 1.65, fontFamily: 'inherit',
                  boxShadow: '0 0 0 4px rgba(0,229,192,0.06)',
                }}
              />
              <div style={{ marginTop: '12px' }}>
                <Button size="lg" fullWidth disabled={!categories.length} onClick={() => navigate('/category/' + (categories[0]?.id ?? ''))}>
                  Продолжить →
                </Button>
              </div>
            </div>
          ) : (
            <SearchBar onClick={() => setBriefOpen(true)} />
          )}
          {!briefOpen && (
            <p style={{ fontSize: '12px', color: TEXT3, fontWeight: 500 }}>
              или выберите категорию ниже
            </p>
          )}
        </div>

        {/* Grid */}
        <div style={{ padding: gridPad }}>
          <div style={{ display: 'grid', gridTemplateColumns: gridCols, gap: isMobile ? '10px' : '14px' }}>
            {loading
              ? Array.from({ length: 8 }, (_, i) => <SkeletonCard key={i} />)
              : categories.map((cat, i) => (
                  <CategoryCard
                    key={cat.id}
                    cat={cat}
                    index={i}
                    onClick={() => navigate(`/category/${cat.id}`)}
                  />
                ))}
          </div>
        </div>

        {/* Mobile examples list */}
        {isMobile && !loading && (
          <div style={{ padding: '0 16px 40px' }}>
            <div style={{ borderTop: `1px solid ${BORDER}`, paddingTop: '24px', marginTop: '8px' }}>
              <div style={{ fontSize: '11px', fontWeight: 700, color: TEXT3, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '12px' }}>
                Послушать примеры
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '8px' }}>
                {EXAMPLE_SONGS.map(ex => (
                  <button
                    key={ex.id}
                    onClick={() => navigate(`/examples/${ex.id}`)}
                    style={{
                      padding: '12px 14px', background: 'rgba(255,255,255,0.03)',
                      border: `1px solid ${BORDER}`, borderRadius: '12px',
                      textAlign: 'left', cursor: 'pointer',
                    }}
                  >
                    <div style={{ fontSize: '13px', fontWeight: 600, color: TEXT2, lineHeight: 1.3, marginBottom: '2px' }}>
                      {ex.title}
                    </div>
                    <div style={{ fontSize: '11px', color: TEXT3 }}>{ex.category}</div>
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}
      </main>

      {/* ── Right panel: desktop only ── */}
      {isDesktop && (
        <aside style={{
          width: PANEL_W, flexShrink: 0,
          borderLeft: `1px solid ${BORDER}`,
          display: 'flex', flexDirection: 'column', overflow: 'hidden',
        }}>
          <div style={{ flex: 1, overflowY: 'auto', padding: '18px 10px 10px' }}>
            <div style={{ fontSize: '10px', fontWeight: 700, color: TEXT3, letterSpacing: '0.08em', textTransform: 'uppercase', padding: '0 12px 10px' }}>
              Популярные категории
            </div>
            {(loading ? [] : categories.slice(0, 8)).map((cat, i) => (
              <SideItem
                key={cat.id}
                title={cat.title}
                sub={`# ${i + 1}`}
                onClick={() => navigate(`/category/${cat.id}`)}
              />
            ))}
          </div>
        </aside>
      )}

    </div>
  )
}
