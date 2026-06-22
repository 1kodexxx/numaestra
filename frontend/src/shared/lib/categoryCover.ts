// Обложки категорий. Локальные WebP лежат в public/categories/<slug>.webp и
// вшиваются в бинарь при сборке. Список ниже — те слаги, для которых картинка
// реально добавлена; для остальных используется URL из БД или сток-заглушка.

const LOCAL_COVERS = new Set<string>([
  'anniversary',
  'apology',
  'baby',
  'birthday',
  'boss',
  'breakup',
  'corporate',
  'doctor',
])

/**
 * Возвращает обложку категории по приоритету:
 *  1) абсолютный http(s)-URL из БД (загруженный через админку) — он всегда
 *     актуальнее зашитого в сборку файла;
 *  2) локальный WebP, если он добавлен для этого слага;
 *  3) null — вызывающий код подставит сток-заглушку.
 */
export function categoryCover(id: string, coverImageUrl?: string): string | null {
  if (coverImageUrl && /^https?:\/\//.test(coverImageUrl)) return coverImageUrl
  if (LOCAL_COVERS.has(id)) return `/categories/${id}.webp`
  return null
}
