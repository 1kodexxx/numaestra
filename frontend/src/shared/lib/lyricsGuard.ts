// Мягкая эвристика против отказа Suno: подсвечивает упоминания РЕАЛЬНЫХ артистов и
// брендов в тексте песни. Suno не генерирует треки с такими упоминаниями (демо и
// платный заказ падают на модерации — см. кейс с «Ludacris»). Список заведомо
// неполный: это подсказка пользователю, а не жёсткая блокировка. Пополняется по
// мере находок в реальных заказах. Мат/жаргон НЕ включаем — Suno его пропускает.
//
// В список кладём только ОДНОЗНАЧНЫЕ имена: без обычных слов (kino, face, каста,
// queen и т.п.), чтобы не ловить ложные срабатывания в обычном тексте.
const RISKY_TERMS: string[] = [
  // — международные артисты —
  'ludacris', 'drake', 'eminem', 'rihanna', 'beyonce', 'beyoncé', 'kanye', 'jay-z',
  'snoop dogg', 'travis scott', 'kendrick lamar', 'post malone', 'ariana grande',
  'taylor swift', 'billie eilish', 'the weeknd', 'bruno mars', 'cardi b',
  'nicki minaj', '50 cent', 'tupac', 'lil wayne', 'michael jackson', 'madonna',
  'metallica', 'nirvana', 'coldplay', 'rammstein',
  // — российские артисты —
  'моргенштерн', 'morgenshtern', 'оксимирон', 'oxxxymiron', 'баста', 'тимати',
  'timati', 'егор крид', 'скриптонит', 'макс корж', 'мияги', 'miyagi', 'элджей',
  'инстасамка', 'slava marlow', 'три дня дождя', 'кровосток', 'ленинград',
  'моргён', 'клава кока', 'джарахов', 'даня милохин', 'зиверт', 'zivert',
  // — бренды —
  'wildberries', 'вайлдберриз', 'ozon', 'озон', 'sberbank', 'сбербанк', 'сбер',
  'iphone', 'айфон', 'samsung', 'nike', 'adidas', 'gucci', 'mercedes',
  'coca-cola', 'кока-кола', 'pepsi', 'пепси', 'газпром', 'gazprom',
]

// detectRiskyTerms возвращает найденные в тексте рискованные упоминания (без
// повторов, в том виде, как заданы в списке). Матчинг по границам слов, с учётом
// кириллицы (JS \b по ней ненадёжен), поэтому нормализуем: всё, кроме букв/цифр/
// дефиса, → пробел, окаймляем пробелами и ищем " term ".
export function detectRiskyTerms(text: string): string[] {
  if (!text.trim()) return []
  const norm =
    ' ' +
    text
      .toLowerCase()
      .replace(/[^\p{L}\p{N}-]+/gu, ' ')
      .replace(/\s+/g, ' ')
      .trim() +
    ' '
  const found: string[] = []
  for (const term of RISKY_TERMS) {
    if (norm.includes(' ' + term + ' ') && !found.includes(term)) {
      found.push(term)
    }
  }
  return found
}
