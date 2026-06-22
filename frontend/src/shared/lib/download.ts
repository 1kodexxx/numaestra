import { showToast } from '@shared/ui'

/**
 * Скачивает файл с принудительным сохранением и именем. Если ресурс
 * кросс-доменный и CORS не разрешён, fetch упадёт — тогда открываем ссылку в
 * новой вкладке (пользователь сохранит вручную).
 */
export async function downloadFile(url: string, filename: string) {
  try {
    const res = await fetch(url, { mode: 'cors' })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const blob = await res.blob()
    const objUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objUrl
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    setTimeout(() => URL.revokeObjectURL(objUrl), 2000)
    showToast('Скачивание началось')
  } catch {
    window.open(url, '_blank', 'noopener')
    showToast('Открыли файл в новой вкладке')
  }
}
