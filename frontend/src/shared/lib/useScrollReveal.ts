import { useEffect, useRef } from 'react'

/**
 * Кинематографичное появление по скроллу. Наблюдает за всеми потомками с
 * атрибутом [data-reveal] внутри контейнера и добавляет класс `is-visible`,
 * когда элемент входит в зону видимости (one-shot).
 *
 * Особенности приложения, которые учитывает хук:
 *  - Страницы скроллятся внутри вложенного контейнера с overflow:auto
 *    (см. App.tsx), а не в окне — поэтому root IntersectionObserver
 *    определяется как ближайший скроллируемый предок, а не viewport.
 *  - Списки (отзывы и т.п.) подгружаются асинхронно — MutationObserver
 *    подхватывает добавленные позже [data-reveal] узлы.
 *  - prefers-reduced-motion: мгновенно показываем всё без анимации.
 *
 * Использование:
 *   const ref = useScrollReveal<HTMLDivElement>()
 *   <div ref={ref}> … <li className="reveal reveal-up" data-reveal> … </li> </div>
 */
export function useScrollReveal<T extends HTMLElement = HTMLElement>(options?: {
  /** Доля элемента в зоне видимости для срабатывания (0..1). */
  threshold?: number
  /** Отступ корня, по умолчанию запуск чуть раньше нижнего края экрана. */
  rootMargin?: string
}) {
  const containerRef = useRef<T | null>(null)
  const threshold = options?.threshold ?? 0.12
  const rootMargin = options?.rootMargin ?? '0px 0px -8% 0px'

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const reveal = (el: Element) => el.classList.add('is-visible')

    // Пользователь предпочитает меньше движения — показываем всё сразу.
    const prefersReduced =
      typeof window !== 'undefined' &&
      window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

    if (prefersReduced || typeof IntersectionObserver === 'undefined') {
      container.querySelectorAll('[data-reveal]').forEach(reveal)
      return
    }

    const scrollRoot = findScrollParent(container)

    const io = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            reveal(entry.target)
            io.unobserve(entry.target)
          }
        }
      },
      { root: scrollRoot, threshold, rootMargin },
    )

    const observeAll = (rootEl: ParentNode) => {
      rootEl.querySelectorAll('[data-reveal]:not(.is-visible)').forEach((el) => io.observe(el))
    }

    observeAll(container)

    // Асинхронно добавленные узлы (например, загруженный список отзывов).
    const mo = new MutationObserver((mutations) => {
      for (const m of mutations) {
        m.addedNodes.forEach((node) => {
          if (!(node instanceof Element)) return
          if (node.matches('[data-reveal]')) io.observe(node)
          observeAll(node)
        })
      }
    })
    mo.observe(container, { childList: true, subtree: true })

    return () => {
      io.disconnect()
      mo.disconnect()
    }
  }, [threshold, rootMargin])

  return containerRef
}

/** Ищет ближайшего скроллируемого предка; null → viewport. */
function findScrollParent(el: HTMLElement): HTMLElement | null {
  let node: HTMLElement | null = el.parentElement
  while (node) {
    const overflowY = getComputedStyle(node).overflowY
    if (overflowY === 'auto' || overflowY === 'scroll') return node
    node = node.parentElement
  }
  return null
}
