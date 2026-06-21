import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCatalog } from '@features/load-catalog'
import { useCreateOrder } from '@features/create-order'
import { EXAMPLE_SONGS } from '@shared/data/examples'
import { Button, TextField, useRipple } from '@shared/ui'
import { ContactModal } from '@widgets/contact-modal'
import { SideItem, PanelHeader, Thumb, PlayOverlay, RankCorner, stockImage } from '@widgets/side-panel'
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

const MOODS = ['Романтика', 'Радость', 'Грусть', 'Ностальгия', 'Энергия', 'Торжественность', 'Юмор', 'Спокойствие', 'Драйв']
const GENRES = ['Поп', 'Баллада', 'Рок', 'Рэп', 'Хип-хоп', 'Джаз', 'R&B', 'Электроника', 'Шансон', 'Акустика', 'Фолк', 'Кантри']
const TEMPOS = ['Медленный', 'Средний', 'Быстрый']
const VOCALS = ['Мужской', 'Женский', 'Дуэт', 'Хор', 'Без вокала']

const SURFACE = '#080808'

interface PromptForm {
  occasion: string
  moods: string[]
  genres: string[]
  tempo: string
  vocal: string
  details: string
  customText: string
}

const EMPTY_FORM: PromptForm = {
  occasion: '', moods: [], genres: [], tempo: '', vocal: '', details: '', customText: '',
}

/* Собираем структурированный, читаемый промпт для Suno из выбранных полей. */
function composeBrief(f: PromptForm): string {
  const lines: string[] = []
  if (f.occasion.trim()) lines.push(`Повод: ${f.occasion.trim()}`)
  if (f.genres.length) lines.push(`Жанр: ${f.genres.join(', ')}`)
  if (f.moods.length) lines.push(`Настроение: ${f.moods.join(', ')}`)
  if (f.tempo) lines.push(`Темп: ${f.tempo}`)
  if (f.vocal) lines.push(`Вокал: ${f.vocal}`)
  if (f.details.trim()) lines.push(`Детали: ${f.details.trim()}`)
  if (f.customText.trim()) lines.push(`Текст песни (использовать дословно):\n${f.customText.trim()}`)
  lines.push('Язык исполнения: русский')
  return lines.join('\n')
}

/* ─── section label with optional hint ─── */
function Section({ label, hint, required, children }: { label: string; hint?: string; required?: boolean; children: React.ReactNode }) {
  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px', marginBottom: '10px' }}>
        <span style={{ fontSize: '12px', color: TEXT2, fontWeight: 500 }}>
          {label}{required && <span style={{ color: '#ef4444', marginLeft: '3px' }}>*</span>}
        </span>
        {hint && <span style={{ fontSize: '11px', color: TEXT3 }}>{hint}</span>}
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>{children}</div>
    </div>
  )
}

/* ─── prompt constructor ─── */
function PromptBuilder({
  form, update, onBack, onSubmit, canSubmit,
}: {
  form: PromptForm
  update: <K extends keyof PromptForm>(key: K, value: PromptForm[K]) => void
  onBack: () => void
  onSubmit: () => void
  canSubmit: boolean
}) {
  const toggleMulti = (key: 'moods' | 'genres', val: string) => {
    const arr = form[key]
    update(key, arr.includes(val) ? arr.filter(x => x !== val) : [...arr, val])
  }
  const preview = composeBrief(form)

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
      <div style={{ fontSize: '22px', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '4px', textAlign: 'left' }}>
        Соберите свою песню
      </div>
      <div style={{ fontSize: '13px', color: TEXT2, marginBottom: '26px', textAlign: 'left', lineHeight: 1.5 }}>
        Выберите стиль, опишите детали и при желании впишите свой текст — мы превратим это в готовый трек.
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '24px', textAlign: 'left' }}>
        <TextField
          label="Для кого и по какому поводу"
          required
          value={form.occasion}
          onChange={(v) => update('occasion', v)}
          placeholder="Например: жене на годовщину свадьбы"
          surfaceColor={SURFACE}
        />

        <Section label="Жанр" hint="можно несколько">
          {GENRES.map(g => (
            <Chip key={g} label={g} selected={form.genres.includes(g)} onClick={() => toggleMulti('genres', g)} />
          ))}
        </Section>

        <Section label="Настроение" hint="можно несколько">
          {MOODS.map(m => (
            <Chip key={m} label={m} selected={form.moods.includes(m)} onClick={() => toggleMulti('moods', m)} />
          ))}
        </Section>

        <Section label="Темп">
          {TEMPOS.map(t => (
            <Chip key={t} label={t} selected={form.tempo === t} onClick={() => update('tempo', form.tempo === t ? '' : t)} />
          ))}
        </Section>

        <Section label="Вокал">
          {VOCALS.map(v => (
            <Chip key={v} label={v} selected={form.vocal === v} onClick={() => update('vocal', form.vocal === v ? '' : v)} />
          ))}
        </Section>

        <TextField
          label="Детали"
          required
          value={form.details}
          onChange={(v) => update('details', v)}
          multiline
          rows={4}
          placeholder="Имена, ваша история, важные слова и моменты, которые обязательно упомянуть..."
          surfaceColor={SURFACE}
          supportingText="Чем больше деталей — тем точнее получится песня."
        />

        <TextField
          label="Свой текст песни (по желанию)"
          value={form.customText}
          onChange={(v) => update('customText', v)}
          multiline
          rows={4}
          placeholder="Впишите строки или припев, которые должны прозвучать дословно..."
          surfaceColor={SURFACE}
        />

        {/* Live preview */}
        <div style={{
          background: 'rgba(0,229,192,0.04)',
          border: '1px solid rgba(0,229,192,0.18)',
          borderRadius: '14px', padding: '16px 18px',
        }}>
          <div style={{ fontSize: '10px', fontWeight: 700, color: ACCENT, letterSpacing: '0.07em', textTransform: 'uppercase', marginBottom: '10px' }}>
            Готовый промпт для Suno
          </div>
          <pre style={{
            whiteSpace: 'pre-wrap', wordBreak: 'break-word',
            fontFamily: 'inherit', fontSize: '13px', lineHeight: 1.6,
            color: 'rgba(255,255,255,0.82)', margin: 0,
          }}>
            {preview}
          </pre>
        </div>
      </div>

      <div style={{ marginTop: '24px' }}>
        {!canSubmit && (
          <div style={{ fontSize: '12px', color: TEXT3, marginBottom: '10px', textAlign: 'center' }}>
            Заполните «Повод» и «Детали», чтобы продолжить
          </div>
        )}
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
  const [form, setForm] = useState<PromptForm>(EMPTY_FORM)
  const [showContact, setShowContact] = useState(false)

  function updateForm<K extends keyof PromptForm>(key: K, value: PromptForm[K]) {
    setForm(prev => ({ ...prev, [key]: value }))
  }

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
    const brief = composeBrief(form)
    await submit({ email, phone, brief, category_id: '', answers: {} })
  }

  const PANEL_W = 240
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
          form={form}
          update={updateForm}
          onBack={closeBuilder}
          onSubmit={() => setShowContact(true)}
          canSubmit={form.occasion.trim().length > 0 && form.details.trim().length > 0}
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
        <div style={{ flex: 1, overflowY: 'auto', minHeight: 0, padding: '18px 10px 12px' }}>
          <PanelHeader icon="🎧" title="Примеры" sub="Послушайте, как звучит" />
          {EXAMPLE_SONGS.map(ex => (
            <SideItem
              key={ex.id}
              title={ex.title}
              sub={ex.category}
              onClick={() => navigate(`/examples/${ex.id}`)}
              leading={(hovered) => (
                <Thumb src={stockImage(ex.id, 'concert,music')} alt={ex.title} active={hovered}>
                  <PlayOverlay active={hovered} />
                </Thumb>
              )}
            />
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
        <div style={{ flex: 1, overflowY: 'auto', minHeight: 0, padding: '18px 10px 12px' }}>
          <PanelHeader icon="🔥" title="Популярное" sub="Выбор пользователей" />
          {categories.slice(0, 8).map((cat, i) => (
            <SideItem
              key={cat.id}
              title={cat.title}
              onClick={() => navigate(`/category/${cat.id}`)}
              leading={(hovered) => (
                <Thumb src={stockImage(cat.id, 'celebration,party')} alt={cat.title} active={hovered}>
                  <RankCorner n={i + 1} />
                </Thumb>
              )}
            />
          ))}
        </div>
      </aside>
    </div>
  )
}
