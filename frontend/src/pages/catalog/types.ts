import type { GenreOption } from '@shared/lib/sunoPrompt'
import { composeCatalogBrief } from '@shared/lib/sunoPrompt'

export interface PromptForm {
  occasion: string
  moods: string[]
  /** sunoValue пресетов из API */
  genres: string[]
  /** пользовательские жанры — только в description */
  customGenres: string[]
  tempo: string
  vocal: string
  details: string
  customText: string
}

export const EMPTY_FORM: PromptForm = {
  occasion: '',
  moods: [],
  genres: [],
  customGenres: [],
  tempo: '',
  vocal: '',
  details: '',
  customText: '',
}

export const MOODS = [
  'Романтика', 'Радость', 'Грусть', 'Ностальгия', 'Энергия',
  'Торжественность', 'Юмор', 'Спокойствие', 'Драйв',
]

export const GENRES_FALLBACK: GenreOption[] = [
  { label: 'Поп', sunoValue: 'modern pop' },
  { label: 'Баллада', sunoValue: 'pop ballad' },
  { label: 'Рок', sunoValue: 'rock' },
  { label: 'Рэп', sunoValue: 'rap' },
  { label: 'Хип-хоп', sunoValue: 'hip hop' },
  { label: 'Джаз', sunoValue: 'smooth jazz' },
  { label: 'R&B', sunoValue: 'contemporary rnb' },
  { label: 'Электроника', sunoValue: 'electronic dance' },
  { label: 'Шансон', sunoValue: 'russian chanson' },
  { label: 'Акустика', sunoValue: 'acoustic guitar' },
  { label: 'Фолк', sunoValue: 'folk' },
  { label: 'Кантри', sunoValue: 'modern country' },
]

export const TEMPOS = ['Медленный', 'Средний', 'Быстрый']
export const VOCALS = ['Мужской', 'Женский', 'Дуэт', 'Хор', 'Без вокала']

export function composeBrief(f: PromptForm, genreOptions: GenreOption[]): string {
  return composeCatalogBrief(f, genreOptions)
}
