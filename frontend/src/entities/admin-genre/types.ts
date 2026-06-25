import type { Genre } from '@entities/genre'

export type AdminGenre = Genre

export interface GenrePayload {
  slug: string
  label: string
  suno_value: string
  sort_order: number
  is_active?: boolean
}

export interface GenreUpdatePayload {
  label: string
  suno_value: string
  sort_order: number
  is_active: boolean
}
