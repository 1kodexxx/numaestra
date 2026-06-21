import { useNavigate } from 'react-router-dom'
import { useCatalog } from '@features/load-catalog'
import { Spinner } from '@shared/ui'

const EMOJI: Record<string, string> = {
  wedding: '💍',
  birthday: '🎂',
  corporate: '🏢',
  roast: '🔥',
}

export function CatalogPage() {
  const { categories, loading, error } = useCatalog()
  const navigate = useNavigate()

  return (
    <div>
      <div className="text-center px-6 pt-20 pb-10 bg-[radial-gradient(ellipse_at_50%_0%,rgba(168,85,247,0.15)_0%,transparent_70%)]">
        <h1 className="text-[clamp(2rem,5vw,3.5rem)] font-extrabold leading-[1.1] mb-4">
          Персональная песня<br />
          <span className="bg-linear-to-br from-accent to-gold bg-clip-text text-transparent">
            для каждого момента
          </span>
        </h1>
        <p className="text-lg text-muted max-w-140 mx-auto mb-10">
          Создаём уникальные треки под ваш запрос — свадьба, день рождения, корпоратив или просто признание в любви.
        </p>
      </div>

      {loading && <div className="text-center py-10"><Spinner /></div>}
      {error && (
        <div className="bg-error/10 border border-error/30 rounded-lg px-4 py-3 text-error max-w-150 mx-auto mb-6">
          {error}
        </div>
      )}

      <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-6 px-6 pb-20 max-w-300 mx-auto">
        {categories.map((cat) => (
          <div
            key={cat.id}
            className="bg-bg2 border border-border rounded-xl overflow-hidden cursor-pointer transition-all duration-300 hover:-translate-y-1 hover:border-accent2 hover:shadow-[0_8px_32px_rgba(124,58,237,0.25)]"
          >
            {cat.cover_image_url
              ? <img src={cat.cover_image_url} alt={cat.title} className="w-full aspect-video object-cover" loading="lazy" />
              : (
                <div className="w-full aspect-video bg-linear-to-br from-bg3 to-bg2 flex items-center justify-center text-5xl">
                  {EMOJI[cat.id] ?? '🎵'}
                </div>
              )
            }
            <div className="p-5">
              <div className="text-lg font-bold mb-2">{cat.title}</div>
              <div className="text-sm text-muted leading-relaxed mb-4">{cat.description}</div>
              <div className="flex flex-wrap gap-1.5 mb-4">
                {cat.seo_tags.slice(0, 3).map((t) => (
                  <span key={t} className="bg-accent/15 border border-accent/30 text-accent px-2.5 py-0.5 rounded-full text-xs">
                    {t}
                  </span>
                ))}
              </div>
              <button
                className="w-full py-2.5 px-5 rounded-lg text-sm font-semibold bg-linear-to-br from-accent2 to-accent text-white border-none cursor-pointer hover:brightness-[1.15] transition-all"
                onClick={() => navigate(`/quiz/${cat.id}`, { state: { title: cat.title } })}
              >
                Заказать песню →
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
