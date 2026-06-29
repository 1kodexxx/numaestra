import { lazy, type ComponentType, type LazyExoticComponent } from 'react'

// Тихое восстановление после деплоя. Фронт разбит на чанки с хешами в имени;
// при выкатке новой версии старые файлы исчезают. Вкладка, открытая со старым
// index.html, при переходе на ленивую страницу пытается загрузить уже
// несуществующий чанк → import() падает. Раньше это показывало экран
// «Что-то пошло не так»; теперь страница один раз сама перезагружается и
// подтягивает свежий билд — пользователь ошибку не видит.

const RELOAD_KEY = 'numaestra_chunk_reload_at'
// Окно защиты от зацикливания: если только что перезагружались, а чанк всё равно
// не грузится (реально отсутствует / нет сети) — больше не перезагружаем,
// показываем обычный фолбэк с кнопкой.
const RELOAD_WINDOW_MS = 12_000

/** Похоже ли исключение на ошибку загрузки динамического чанка (а не баг кода). */
export function isChunkLoadError(err: unknown): boolean {
  const msg = err instanceof Error ? `${err.name} ${err.message}` : String(err)
  return /loading chunk|chunkloaderror|dynamically imported module|module script failed|failed to fetch dynamically|error loading dynamically imported/i.test(
    msg,
  )
}

/**
 * Если ошибка — это устаревший чанк, перезагружает страницу ОДИН раз и возвращает
 * true. Повторно в пределах RELOAD_WINDOW_MS не перезагружает (защита от петли).
 * Для не-чанковых ошибок возвращает false (их должен обработать ErrorBoundary).
 */
export function reloadOnceForChunkError(err: unknown): boolean {
  if (!isChunkLoadError(err)) return false
  try {
    const last = Number(sessionStorage.getItem(RELOAD_KEY) || 0)
    if (Date.now() - last < RELOAD_WINDOW_MS) return false
    sessionStorage.setItem(RELOAD_KEY, String(Date.now()))
  } catch {
    // sessionStorage недоступен (приватный режим) — всё равно пробуем перезагрузиться.
  }
  window.location.reload()
  return true
}

/**
 * Drop-in замена React.lazy: при падении динамического импорта из-за устаревшего
 * чанка перезагружает страницу вместо проброса ошибки в ErrorBoundary. Прочие
 * ошибки импорта пробрасываются как обычно.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- зеркалим сигнатуру React.lazy для drop-in-замены
export function lazyWithReload<T extends ComponentType<any>>(
  factory: () => Promise<{ default: T }>,
): LazyExoticComponent<T> {
  return lazy(() =>
    factory().catch((err: unknown) => {
      if (reloadOnceForChunkError(err)) {
        // Страница уже перезагружается — отдаём «вечный» промис, чтобы Suspense
        // остался в фолбэке и ничего не отрендерил до перезагрузки.
        return new Promise<{ default: T }>(() => {})
      }
      throw err
    }),
  )
}
