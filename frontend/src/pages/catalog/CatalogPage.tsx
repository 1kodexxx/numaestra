import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCatalog } from '@features/load-catalog'
import { useCreateOrder } from '@features/create-order'
import { EXAMPLE_SONGS } from '@shared/data/examples'
import { Button, TextField, useRipple } from '@shared/ui'
import { ContactModal } from '@widgets/contact-modal'
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
  ['валентин', '💝'], ['valentine', '💝'],
  ['8 март', '🌷'], ['march8', '🌷'], ['женск', '🌷'],
  ['повышен', '📈'], ['promotion', '📈'], ['карьер', '📈'],
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
function SearchBar({ onOpen }: { onOpen: () => void }) {
  return (
    <button
      onClick={onOpen}
      onMouseEnter={(e) => {
        onOpen()
        e.currentTarget.style.borderColor = 'rgba(0,229,192,0.35)'
        e.currentTarget.style.background = 'rgba(255,255,255,0.045)'
        e.currentTarget.style.boxShadow = '0 0 0 4px rgba(0,229,192,0.05)'
      }}
      onFocus={onOpen}
      style={{
        display: 'flex', alignItems: 'center', gap: '12px',
        width: '100%', maxWidth: '580px',
        background: 'rgba(255,255,255,0.03)',
        border: '1px solid rgba(255,255,255,0.08)',
        borderRadius: '14px', padding: '15px 20px',
        cursor: 'text', transition: 'all 0.2s', textAlign: 'left',
      }}
    >
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none"
        stroke="rgba(255,255,255,0.25)" strokeWidth="2.5" strokeLinecap="round">
        <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
      </svg>
      <span style={{ fontSize: '14px', color: 'rgba(255,255,255,0.25)', fontWeight: 400, letterSpacing: '0.01em' }}>
        Опишите вашу песню к Suno или выберите категорию...
      </span>
    </button>
  )
}

/* ─── chip (single-select) ─── */
function Chip({ label, selected, onClick }: { label: string; selected: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      style={{
        padding: '9px 16px', borderRadius: '20px',
        background: selected ? 'rgba(0,229,192,0.14)' : 'rgba(255,255,255,0.04)',
        border: `1px solid ${selected ? 'rgba(0,229,192,0.5)' : 'rgba(255,255,255,0.1)'}`,
        color: selected ? ACCENT : 'rgba(255,255,255,0.6)',
        fontSize: '13px', fontWeight: selected ? 600 : 400,
        cursor: 'pointer', transition: 'all 0.15s',
      }}
      onMouseEnter={(e) => { if (!selected) { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.2)'; e.currentTarget.style.color = '#fff' } }}
      onMouseLeave={(e) => { if (!selected) { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.1)'; e.currentTarget.style.color = 'rgba(255,255,255,0.6)' } }}
    >
      {label}
    </button>
  )
}

const MOODS = ['Романтика', 'Радость', 'Энергия', 'Ностальгия', 'Юмор', 'Торжественность']
const GENRES = ['Поп', 'Баллада', 'Рок', 'Рэп', 'Джаз', 'Электроника', 'Шансон', 'Акустика']

function composeBrief(occasion: string, mood: string, genre: string, details: string): string {
  const parts: string[] = []
  if (occasion.trim()) parts.push(occasion.trim())
  if (mood || genre) parts.push(`Настроение и стиль: ${[mood, genre].filter(Boolean).join(', ')}`)
  if (details.trim()) parts.push(details.trim())
  return parts.join('. ') || 'Персональная песня на заказ'
}

/* ─── prompt constructor ─── */
function PromptBuilder({
  occasion, setOccasion, mood, setMood, genre, setGenre, details, setDetails, onBack, onSubmit, canSubmit,
}: {
  occasion: string; setOccasion: (v: string) => void
  mood: string; setMood: (v: string) => void
  genre: string; setGenre: (v: string) => void
  details: string; setDetails: (v: string) => void
  onBack: () => void
  onSubmit: () => void
  canSubmit: boolean
}) {
  return (
    <div style={{ width: '100%', maxWidth: '640px' }} className="fade-in">
      <button
        onClick={onBack}
        style={{ background: 'none', border: 'none', color: TEXT2, fontSize: '13px', cursor: 'pointer', marginBottom: '18px', padding: 0 }}
        onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
        onMouseLeave={(e) => { e.currentTarget.style.color = TEXT2 }}
      >
        ← К категориям
      </button>

      <div style={{ display: 'inline-flex', alignItems: 'center', gap: '7px', marginBottom: '12px' }}>
        <span style={{ fontSize: '12px' }}>🎛️</span>
        <span style={{ fontSize: '11px', fontWeight: 700, color: ACCENT, letterSpacing: '0.06em', textTransform: 'uppercase' }}>
          Конструктор промпта для Suno
        </span>
      </div>
      <div style={{ fontSize: '20px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '4px', textAlign: 'left' }}>
        Соберите свою песню
      </div>
      <div style={{ fontSize: '13px', color: TEXT2, marginBottom: '24px', textAlign: 'left' }}>
        Никаких готовых категорий — только ваш повод, настроение и детали
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '22px', textAlign: 'left' }}>
        <TextField
          label="Для кого и по какому поводу"
          value={occasion}
          onChange={setOccasion}
          placeholder="Например: жене на годовщину свадьбы"
          surfaceColor="#080808"
        />

        <div>
          <div style={{ fontSize: '12px', color: TEXT2, marginBottom: '10px', fontWeight: 500 }}>Настроение</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
            {MOODS.map(m => (
              <Chip key={m} label={m} selected={mood === m} onClick={() => setMood(mood === m ? '' : m)} />
            ))}
          </div>
        </div>

        <div>
          <div style={{ fontSize: '12px', color: TEXT2, marginBottom: '10px', fontWeight: 500 }}>Жанр</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
            {GENRES.map(g => (
              <Chip key={g} label={g} selected={genre === g} onClick={() => setGenre(genre === g ? '' : g)} />
            ))}
          </div>
        </div>

        <TextField
          label="Детали (необязательно)"
          value={details}
          onChange={setDetails}
          multiline
          rows={4}
          placeholder="Имена, общие воспоминания, шутки, особые слова..."
          surfaceColor="#080808"
        />
      </div>

      <div style={{ marginTop: '24px' }}>
        <Button size="lg" fullWidth disabled={!canSubmit} onClick={onSubmit}>
          Продолжить — 2 000 ₽ →
        </Button>
      </div>
    </div>
  )
}

/* ─── category card ─── */
function CategoryCard({ cat, index, onClick }: { cat: Category; index: number; onClick: () => void }) {
  const [h, setH] = useState(false)
  const icon = getIcon(cat, index)
  const { onPointerDown, rippleEl } = useRipple('rgba(0,229,192,0.5)')

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
        background: h ? 'rgba(0,229,192,0.06)' : 'rgba(255,255,255,0.02)',
        border: `1.5px solid ${h ? 'rgba(0,229,192,0.5)' : 'rgba(255,255,255,0.1)'}`,
        borderRadius: '22px',
        cursor: 'pointer',
        transform: h ? 'translateY(-4px)' : 'translateY(0)',
        boxShadow: h ? '0 16px 36px rgba(0,229,192,0.12)' : 'none',
        transition: 'all 0.24s cubic-bezier(0.34, 1.4, 0.64, 1)',
        overflow: 'hidden',
        position: 'relative',
      }}
    >
      {/* icon badge */}
      <div style={{
        width: '52px', height: '52px', borderRadius: '50%',
        background: h ? 'rgba(0,229,192,0.14)' : 'rgba(255,255,255,0.05)',
        border: `1px solid ${h ? 'rgba(0,229,192,0.4)' : 'rgba(255,255,255,0.12)'}`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        transition: 'all 0.24s', position: 'relative',
        transform: h ? 'scale(1.08)' : 'scale(1)',
      }}>
        <span style={{ fontSize: '26px', lineHeight: 1, filter: h ? 'none' : 'grayscale(0.15)' }}>{icon}</span>
      </div>

      <span style={{
        fontSize: '13px', fontWeight: 700,
        color: h ? '#00e5c0' : 'rgba(255,255,255,0.85)',
        lineHeight: 1.25, letterSpacing: '-0.01em', textAlign: 'center',
        position: 'relative', transition: 'color 0.2s',
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
  const [occasion, setOccasion] = useState('')
  const [mood, setMood] = useState('')
  const [genre, setGenre] = useState('')
  const [details, setDetails] = useState('')
  const [showContact, setShowContact] = useState(false)

  // Когда конструктор закрывается, строка поиска возвращается на то же место
  // экрана, где уже стоит курсор. Браузер пересчитывает hover под неподвижным
  // указателем и мгновенно шлёт mouseenter новому элементу — конструктор тут же
  // открывался бы обратно. Блокируем повторное открытие по hover, пока
  // пользователь не подвинет мышь по-настоящему.
  const suppressHoverRef = useRef(false)

  function openBuilder() {
    if (suppressHoverRef.current) return
    setBriefOpen(true)
  }

  function closeBuilder() {
    setBriefOpen(false)
    suppressHoverRef.current = true
    const clear = () => {
      suppressHoverRef.current = false
      document.removeEventListener('mousemove', clear)
    }
    document.addEventListener('mousemove', clear)
  }
  const { loading: submitting, error: submitError, submit } = useCreateOrder()

  async function handleCustomOrder(email: string, phone: string) {
    const brief = composeBrief(occasion, mood, genre, details)
    await submit({ email, phone, brief, category_id: '', answers: {} })
  }

  const PANEL_W = 210
  // auto-fill держит карточки компактными (~180px) и заполняет ширину колонками,
  // вместо 4 гигантских карточек на ультрашироком экране.
  const gridCols = isMobile ? 'repeat(2, 1fr)' : 'repeat(auto-fill, minmax(180px, 1fr))'
  const gridPad = isMobile ? '4px 16px 32px' : isTablet ? '4px 28px 40px' : '4px 40px 40px'
  const heroPad = isMobile ? '24px 16px 8px' : isTablet ? '32px 28px 10px' : '40px 40px 12px'
  const searchPad = isMobile ? '16px 16px 12px' : '20px 40px 18px'

  const centerInner = (
    <>
      {/* Hero */}
      <div style={{ padding: heroPad }}>
        <Hero compact={isMobile} />
      </div>

      {/* Category chips (mobile/tablet) */}
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

      {/* Search bar (opens the constructor on hover/focus) */}
      <div style={{ padding: searchPad, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '10px' }}>
        <SearchBar onOpen={openBuilder} />
        <p style={{ fontSize: '12px', color: TEXT3, fontWeight: 500 }}>
          или выберите категорию ниже
        </p>
      </div>

      {/* Category grid */}
      <div style={{ padding: gridPad }}>
        <div style={{ display: 'grid', gridTemplateColumns: gridCols, gap: isMobile ? '10px' : '14px', maxWidth: 1280, margin: '0 auto' }}>
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

      {/* Examples list (mobile) */}
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
    </>
  )

  // Конструктор промпта — НЕПРОЗРАЧНЫЙ полноэкранный оверлей. Полностью
  // перекрывает каталог (на любом размере экрана), поэтому карточкам не нужны
  // хрупкие collapse-анимации, завязанные на flex/maxHeight.
  const constructorOverlay = briefOpen && (
    <div
      className="fade-in"
      style={{
        position: 'fixed', top: 60, left: 0, right: 0, bottom: 0, zIndex: 40,
        background: '#080808',
        display: 'flex', alignItems: 'flex-start', justifyContent: 'center',
        overflowY: 'auto',
        padding: isMobile ? '20px 16px 40px' : '32px 24px',
      }}
    >
      <div style={{ width: '100%', maxWidth: 640, margin: 'auto' }}>
        <PromptBuilder
          occasion={occasion} setOccasion={setOccasion}
          mood={mood} setMood={setMood}
          genre={genre} setGenre={setGenre}
          details={details} setDetails={setDetails}
          onBack={closeBuilder}
          onSubmit={() => setShowContact(true)}
          canSubmit={occasion.trim().length > 0}
        />
      </div>
    </div>
  )

  const modals = (
    <>
      {showContact && (
        <ContactModal
          loading={submitting}
          error={submitError}
          onClose={() => setShowContact(false)}
          onSubmit={handleCustomOrder}
        />
      )}
      {constructorOverlay}
    </>
  )

  /* ── Mobile / tablet: один простой скролл-контейнер (без вложенного flex) ── */
  if (!isDesktop) {
    return (
      <>
        {modals}
        <div style={{ height: 'calc(100dvh - 60px)', overflowY: 'auto', WebkitOverflowScrolling: 'touch' } as React.CSSProperties}>
          {centerInner}
        </div>
      </>
    )
  }

  /* ── Desktop: 3-колоночный shell с независимыми скроллами ── */
  return (
    <div style={{ display: 'flex', height: 'calc(100dvh - 60px)', overflow: 'hidden' }}>
      {modals}

      {/* Left panel: examples */}
      <aside style={{
        width: PANEL_W, flexShrink: 0,
        borderRight: `1px solid ${BORDER}`,
        display: 'flex', flexDirection: 'column', overflow: 'hidden',
      }}>
        <div style={{ flex: 1, overflowY: 'auto', minHeight: 0, padding: '18px 10px 10px' }}>
          <div style={{ fontSize: '10px', fontWeight: 700, color: TEXT3, letterSpacing: '0.08em', textTransform: 'uppercase', padding: '0 12px 10px' }}>
            Послушать примеры
          </div>
          {EXAMPLE_SONGS.map(ex => (
            <SideItem key={ex.id} title={ex.title} sub={ex.category} onClick={() => navigate(`/examples/${ex.id}`)} />
          ))}
        </div>
      </aside>

      {/* Center */}
      <main style={{ flex: 1, display: 'flex', flexDirection: 'column', overflowY: 'auto', minWidth: 0, minHeight: 0 }}>
        {centerInner}
      </main>

      {/* Right panel: top categories */}
      <aside style={{
        width: PANEL_W, flexShrink: 0,
        borderLeft: `1px solid ${BORDER}`,
        display: 'flex', flexDirection: 'column', overflow: 'hidden',
      }}>
        <div style={{ flex: 1, overflowY: 'auto', minHeight: 0, padding: '18px 10px 10px' }}>
          <div style={{ fontSize: '10px', fontWeight: 700, color: TEXT3, letterSpacing: '0.08em', textTransform: 'uppercase', padding: '0 12px 10px' }}>
            Популярные категории
          </div>
          {categories.slice(0, 8).map((cat, i) => (
            <SideItem
              key={cat.id}
              title={cat.title}
              sub={`# ${i + 1}`}
              onClick={() => navigate(`/category/${cat.id}`)}
            />
          ))}
        </div>
      </aside>
    </div>
  )
}
