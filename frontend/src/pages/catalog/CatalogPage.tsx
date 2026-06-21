import { useNavigate } from 'react-router-dom'
import { useCatalog } from '@features/load-catalog'
import { Spinner } from '@shared/ui'
import styles from './CatalogPage.module.css'

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
      <div className={styles.hero}>
        <h1>Персональная песня<br /><span>для каждого момента</span></h1>
        <p>Создаём уникальные треки под ваш запрос — свадьба, день рождения, корпоратив или просто признание в любви.</p>
      </div>

      {loading && <div className={styles.center}><Spinner /></div>}
      {error && <div className={styles.error}>{error}</div>}

      <div className={styles.grid}>
        {categories.map((cat) => (
          <div key={cat.id} className={styles.card}>
            {cat.cover_image_url
              ? <img src={cat.cover_image_url} alt={cat.title} className={styles.cardImg} loading="lazy" />
              : <div className={styles.cardImgPlaceholder}>{EMOJI[cat.id] ?? '🎵'}</div>
            }
            <div className={styles.cardBody}>
              <div className={styles.cardTitle}>{cat.title}</div>
              <div className={styles.cardDesc}>{cat.description}</div>
              <div className={styles.cardTags}>
                {cat.seo_tags.slice(0, 3).map((t) => (
                  <span key={t} className={styles.tag}>{t}</span>
                ))}
              </div>
              <button
                className={styles.btnPrimary}
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
