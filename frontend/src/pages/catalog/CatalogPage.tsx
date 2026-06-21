import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCatalog } from '@features/load-catalog'
import { EXAMPLE_SONGS } from '@shared/data/examples'
import type { Category } from '@entities/category'

const CYAN = '#00e5c0'
const DARK = '#0d0d0d'

function ExampleCard({ title, onClick }: { title: string; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="w-full rounded-xl text-left transition-all cursor-pointer"
      style={{
        background: CYAN, color: DARK, border: 'none',
        padding: '18px 14px', marginBottom: '8px',
        fontWeight: 600, fontSize: '14px',
      }}
      onMouseEnter={(e) => { e.currentTarget.style.filter = 'brightness(1.1)' }}
      onMouseLeave={(e) => { e.currentTarget.style.filter = '' }}
    >
      {title}
    </button>
  )
}

function CategorySideCard({ title, onClick }: { title: string; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="w-full rounded-xl text-left transition-all cursor-pointer"
      style={{
        background: CYAN, color: DARK, border: 'none',
        padding: '18px 14px', marginBottom: '8px',
        fontWeight: 600, fontSize: '14px',
      }}
      onMouseEnter={(e) => { e.currentTarget.style.filter = 'brightness(1.1)' }}
      onMouseLeave={(e) => { e.currentTarget.style.filter = '' }}
    >
      {title}
    </button>
  )
}

function CategoryGridCard({ cat, onClick }: { cat: Category; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="rounded-xl transition-all cursor-pointer w-full aspect-square flex items-center justify-center"
      style={{
        background: CYAN, color: DARK, border: 'none',
        fontWeight: 700, fontSize: '15px',
        textAlign: 'center', padding: '12px',
      }}
      onMouseEnter={(e) => { e.currentTarget.style.filter = 'brightness(1.12)'; e.currentTarget.style.transform = 'scale(1.03)' }}
      onMouseLeave={(e) => { e.currentTarget.style.filter = ''; e.currentTarget.style.transform = '' }}
    >
      {cat.title}
    </button>
  )
}

function CenterInput({ onFocus }: { onFocus: () => void }) {
  return (
    <div className="flex justify-center px-6 py-5">
      <div
        className="w-full max-w-xl rounded-2xl flex items-center gap-3 cursor-text"
        style={{
          background: '#fff', padding: '16px 22px',
          boxShadow: '0 2px 20px rgba(0,0,0,0.3)',
        }}
        onClick={onFocus}
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="#999">
          <path d="M21.71 20.29 18 16.61A9 9 0 1 0 16.61 18l3.68 3.68a1 1 0 0 0 1.42-1.39ZM11 18a7 7 0 1 1 7-7 7 7 0 0 1-7 7Z"/>
        </svg>
        <span style={{ color: '#999', fontSize: '15px', flex: 1 }}>
          Опишите песню или выберите категорию ниже...
        </span>
      </div>
    </div>
  )
}

export function CatalogPage() {
  const { categories, loading } = useCatalog()
  const navigate = useNavigate()
  const [briefInput, setBriefInput] = useState('')
  const [inputOpen, setInputOpen] = useState(false)

  const topCategories = categories.slice(0, 6)
  const gridCategories = categories

  function openCategory(catId: string) {
    navigate(`/category/${catId}`)
  }

  function openExample(id: string) {
    navigate(`/examples/${id}`)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center" style={{ height: 'calc(100vh - 56px)' }}>
        <div className="w-8 h-8 rounded-full border-2 border-t-transparent spin-anim" style={{ borderColor: `${CYAN} transparent transparent transparent` }} />
      </div>
    )
  }

  return (
    <div className="flex" style={{ height: 'calc(100vh - 56px)' }}>
      {/* Left panel – example songs */}
      <aside
        className="flex flex-col p-4"
        style={{ width: '210px', borderRight: '1px solid #252525', flexShrink: 0, overflowY: 'auto' }}
      >
        <div className="flex-1">
          {EXAMPLE_SONGS.map((ex) => (
            <ExampleCard key={ex.id} title={ex.title} onClick={() => openExample(ex.id)} />
          ))}
        </div>
        <p className="text-xs mt-4 text-center" style={{ color: '#555' }}>Список из примеров работ</p>
      </aside>

      {/* Center */}
      <main className="flex-1 flex flex-col overflow-y-auto">
        {inputOpen ? (
          /* Expanded brief input */
          <div className="flex flex-col items-center px-8 py-6 gap-4">
            <div className="w-full max-w-2xl">
              <button
                className="text-sm mb-4 flex items-center gap-1.5 transition-colors"
                style={{ background: 'none', border: 'none', color: '#777', cursor: 'pointer' }}
                onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
                onMouseLeave={(e) => { e.currentTarget.style.color = '#777' }}
                onClick={() => setInputOpen(false)}
              >
                ← Вернуться к категориям
              </button>
              <textarea
                autoFocus
                placeholder="Опишите свою песню... Например: весёлая песня на день рождения лучшей подруги, которая любит котов и путешествия"
                value={briefInput}
                onChange={(e) => setBriefInput(e.target.value)}
                rows={8}
                className="w-full rounded-2xl resize-none outline-none text-base"
                style={{
                  background: '#fff', color: '#0d0d0d',
                  padding: '24px', fontSize: '16px',
                  border: 'none', lineHeight: 1.6,
                  boxShadow: '0 4px 32px rgba(0,0,0,0.4)',
                }}
              />
              <p className="text-sm mt-3 text-center" style={{ color: '#555' }}>
                Или выберите категорию ниже — мы зададим уточняющие вопросы
              </p>
            </div>
          </div>
        ) : (
          <CenterInput onFocus={() => setInputOpen(true)} />
        )}

        {/* Category grid */}
        {loading ? null : (
          <div className="px-6 pb-6">
            <div
              className="grid gap-3"
              style={{ gridTemplateColumns: 'repeat(4, 1fr)' }}
            >
              {gridCategories.map((cat) => (
                <CategoryGridCard key={cat.id} cat={cat} onClick={() => openCategory(cat.id)} />
              ))}
            </div>
          </div>
        )}
      </main>

      {/* Right panel – top categories */}
      <aside
        className="flex flex-col p-4"
        style={{ width: '210px', borderLeft: '1px solid #252525', flexShrink: 0, overflowY: 'auto' }}
      >
        <div className="flex-1">
          {topCategories.map((cat, i) => (
            <CategorySideCard
              key={cat.id}
              title={`Категория #${i + 1}`}
              onClick={() => openCategory(cat.id)}
            />
          ))}
        </div>
        <p className="text-xs mt-4 text-center" style={{ color: '#555' }}>Список из топовых категорий</p>
      </aside>
    </div>
  )
}
