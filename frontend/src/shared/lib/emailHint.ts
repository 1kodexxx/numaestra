// Подсказка об опечатке в домене email. Ловит частые промахи («gmial.com»,
// «yandex.ry», «mail.ri», «gmail.con») и предлагает исправление. Работает офлайн
// по небольшой таблице популярных доменов + расстоянию Левенштейна — без сети и
// сторонних зависимостей. Если домен уже корректный (есть в таблице) — молчит.

// Популярные у российской аудитории почтовые домены. Если введённый домен похож
// на один из них (но не равен ему), считаем это опечаткой и предлагаем исправить.
const POPULAR_DOMAINS = [
  'gmail.com',
  'yandex.ru',
  'ya.ru',
  'mail.ru',
  'bk.ru',
  'inbox.ru',
  'list.ru',
  'internet.ru',
  'outlook.com',
  'hotmail.com',
  'yahoo.com',
  'icloud.com',
  'me.com',
  'rambler.ru',
  'proton.me',
  'protonmail.com',
]

// Высокочастотные опечатки, для которых не хотим полагаться на эвристику
// расстояния, — однозначное соответствие.
const EXPLICIT_TYPOS: Record<string, string> = {
  'gmail.con': 'gmail.com',
  'gmail.co': 'gmail.com',
  'gmail.cm': 'gmail.com',
  'gmail.ru': 'gmail.com',
  'gmial.com': 'gmail.com',
  'gmai.com': 'gmail.com',
  'gmaill.com': 'gmail.com',
  'gmail.comm': 'gmail.com',
  'gnail.com': 'gmail.com',
  'yandex.ru.com': 'yandex.ru',
  'yandex.com': 'yandex.ru',
  'yandex.ry': 'yandex.ru',
  'yandex.com.ru': 'yandex.ru',
  'yndex.ru': 'yandex.ru',
  'yanex.ru': 'yandex.ru',
  'mail.ri': 'mail.ru',
  'mail.ru.com': 'mail.ru',
  'mai.ru': 'mail.ru',
  'maill.ru': 'mail.ru',
  'outlook.con': 'outlook.com',
  'hotmail.con': 'hotmail.com',
  'yahoo.con': 'yahoo.com',
  'icloud.com': 'icloud.com',
  'icloud.con': 'icloud.com',
}

// Классическое расстояние Левенштейна (вставка/удаление/замена), итеративно.
function editDistance(a: string, b: string): number {
  const m = a.length
  const n = b.length
  if (m === 0) return n
  if (n === 0) return m
  let prev = Array.from({ length: n + 1 }, (_, i) => i)
  let curr = new Array<number>(n + 1)
  for (let i = 1; i <= m; i++) {
    curr[0] = i
    for (let j = 1; j <= n; j++) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1
      curr[j] = Math.min(prev[j] + 1, curr[j - 1] + 1, prev[j - 1] + cost)
    }
    ;[prev, curr] = [curr, prev]
  }
  return prev[n]
}

/**
 * Возвращает исправленный email, если домен похож на популярный с опечаткой,
 * либо null, если подсказка не нужна (домен корректный, адрес слишком короткий
 * или ни на что не похож). Локальную часть (до «@») не трогаем.
 */
export function suggestEmailFix(raw: string): string | null {
  const email = raw.trim().toLowerCase()
  const at = email.lastIndexOf('@')
  if (at <= 0 || at === email.length - 1) return null

  const local = raw.trim().slice(0, raw.trim().lastIndexOf('@'))
  const domain = email.slice(at + 1)

  // Домен уже корректный — не мешаем.
  if (POPULAR_DOMAINS.includes(domain)) return null

  // Явная таблица частых опечаток — самый надёжный случай.
  if (EXPLICIT_TYPOS[domain]) {
    return `${local}@${EXPLICIT_TYPOS[domain]}`
  }

  // Иначе ищем ближайший популярный домен по расстоянию Левенштейна.
  let best: string | null = null
  let bestDist = Infinity
  for (const candidate of POPULAR_DOMAINS) {
    const d = editDistance(domain, candidate)
    if (d < bestDist) {
      bestDist = d
      best = candidate
    }
  }
  if (!best || bestDist === 0) return null

  // Консервативный порог: расстояние 1 — почти всегда опечатка; расстояние 2
  // принимаем только для длинных доменов (≥9 символов), чтобы не «исправлять»
  // короткие непохожие домены (bk.ru → mail.ru и т.п.).
  const accept = bestDist === 1 || (bestDist === 2 && best.length >= 9)
  if (!accept) return null

  return `${local}@${best}`
}
