/** Инжект JSON-LD в <head> (удаляет предыдущий скрипт с тем же id). */
export function injectJsonLd(id: string, data: unknown): void {
  if (typeof document === 'undefined') return
  document.getElementById(id)?.remove()
  const s = document.createElement('script')
  s.id = id
  s.type = 'application/ld+json'
  s.text = JSON.stringify(data)
  document.head.appendChild(s)
}

export function breadcrumbJsonLd(items: { name: string; path: string }[]): object {
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: items.map((item, i) => ({
      '@type': 'ListItem',
      position: i + 1,
      name: item.name,
      item: origin + item.path,
    })),
  }
}

export function aggregateRatingJsonLd(avg: number, count: number): object {
  return {
    '@context': 'https://schema.org',
    '@type': 'Product',
    name: 'Персональная песня Numaestra',
    aggregateRating: {
      '@type': 'AggregateRating',
      ratingValue: avg.toFixed(1),
      reviewCount: count,
      bestRating: 5,
      worstRating: 1,
    },
  }
}
