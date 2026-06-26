import { useEffect, useRef } from 'react'

/** Удерживает Tab внутри контейнера (модалки, mobile menu). */
export function useFocusTrap(active: boolean) {
  const ref = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!active) return
    const root = ref.current
    if (!root) return

    const focusables = () =>
      Array.from(
        root.querySelectorAll<HTMLElement>(
          'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])',
        ),
      ).filter((el) => !el.hasAttribute('disabled') && el.tabIndex !== -1)

    const first = focusables()[0]
    first?.focus()

    function onKey(e: KeyboardEvent) {
      if (e.key !== 'Tab') return
      const items = focusables()
      if (items.length === 0) return
      const i = items.indexOf(document.activeElement as HTMLElement)
      if (e.shiftKey) {
        if (i <= 0) {
          e.preventDefault()
          items[items.length - 1]?.focus()
        }
      } else if (i === items.length - 1) {
        e.preventDefault()
        items[0]?.focus()
      }
    }

    root.addEventListener('keydown', onKey)
    return () => root.removeEventListener('keydown', onKey)
  }, [active])

  return ref
}
