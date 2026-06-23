import { apiFetch } from '@shared/api'

// PublicExample — форма примера из публичного API (snake_case, как отдаёт бэкенд).
export interface PublicExample {
  id: string
  title: string
  category: string
  description: string
  mood: string
  audio_url: string
  cover_url: string
}

export const exampleApi = {
  list() {
    return apiFetch<PublicExample[]>('/examples/')
  },
}
