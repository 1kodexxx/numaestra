// Лёгкий императивный toast без зависимостей и React-дерева: создаёт элемент в
// body, плавно показывает и сам убирает. Уважает prefers-reduced-motion.

function ensureHost(): HTMLElement {
  let host = document.getElementById('toast-host')
  if (!host) {
    host = document.createElement('div')
    host.id = 'toast-host'
    host.style.cssText =
      'position:fixed;left:0;right:0;bottom:24px;z-index:300;display:flex;flex-direction:column;align-items:center;gap:8px;pointer-events:none'
    document.body.appendChild(host)
  }
  return host
}

export function showToast(message: string) {
  const host = ensureHost()
  const el = document.createElement('div')
  el.textContent = message
  el.style.cssText =
    'background:#141414;border:1px solid rgba(0,229,192,0.4);color:#fff;' +
    "font:600 13px Inter,system-ui,sans-serif;padding:10px 16px;border-radius:12px;" +
    'box-shadow:0 10px 28px rgba(0,0,0,0.45);opacity:0;transform:translateY(8px);' +
    'transition:opacity .2s ease,transform .2s ease;max-width:90vw'
  host.appendChild(el)
  requestAnimationFrame(() => { el.style.opacity = '1'; el.style.transform = 'translateY(0)' })
  setTimeout(() => {
    el.style.opacity = '0'
    el.style.transform = 'translateY(8px)'
    setTimeout(() => el.remove(), 220)
  }, 1900)
}

/** Копирует текст в буфер обмена и показывает toast. */
export async function copyText(text: string, message = 'Скопировано') {
  try {
    await navigator.clipboard.writeText(text)
    showToast(message)
  } catch {
    showToast('Не удалось скопировать')
  }
}
