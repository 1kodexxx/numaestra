import { useEffect } from 'react'

const SITE = 'Numaestra'
const DEFAULT_DESC =
  'AI-студия Numaestra создаёт уникальную песню под ваш повод. Опишите идею — получите 4 готовые версии трека за 10 минут.'

interface SeoOptions {
  title: string
  description?: string
  /** Абсолютный или относительный путь к OG-картинке. */
  image?: string
  /** Не индексировать страницу (служебные экраны). */
  noindex?: boolean
}

function setMeta(attr: 'name' | 'property', key: string, content: string) {
  let el = document.head.querySelector<HTMLMetaElement>(`meta[${attr}="${key}"]`)
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute(attr, key)
    document.head.appendChild(el)
  }
  el.setAttribute('content', content)
}

function setLink(rel: string, href: string) {
  let el = document.head.querySelector<HTMLLinkElement>(`link[rel="${rel}"]`)
  if (!el) {
    el = document.createElement('link')
    el.setAttribute('rel', rel)
    document.head.appendChild(el)
  }
  el.setAttribute('href', href)
}

/**
 * Обновляет <title>, description, canonical и OG/Twitter-теги под конкретную
 * страницу. Абсолютные URL берутся из текущего origin — корректно при любом
 * домене. Googlebot исполняет JS и видит обновлённые значения.
 */
export function useSeo({ title, description, image, noindex }: SeoOptions) {
  useEffect(() => {
    const fullTitle = title.includes(SITE) ? title : `${title} — ${SITE}`
    const desc = description || DEFAULT_DESC
    const origin = window.location.origin
    const url = origin + window.location.pathname
    const img = image ? (image.startsWith('http') ? image : origin + image) : origin + '/og-image.png'

    document.title = fullTitle
    setMeta('name', 'description', desc)
    setMeta('name', 'robots', noindex ? 'noindex, nofollow' : 'index, follow, max-image-preview:large')

    setMeta('property', 'og:title', fullTitle)
    setMeta('property', 'og:description', desc)
    setMeta('property', 'og:url', url)
    setMeta('property', 'og:image', img)
    setMeta('name', 'twitter:title', fullTitle)
    setMeta('name', 'twitter:description', desc)
    setMeta('name', 'twitter:image', img)

    setLink('canonical', url)
  }, [title, description, image, noindex])
}
