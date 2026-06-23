export interface AdminExample {
  id: string
  title: string
  category: string
  description: string
  mood: string
  audio_url: string
  cover_url: string
  sort_order: number
  is_active: boolean
}

export interface ExamplePayload {
  id?: string
  title: string
  category: string
  description: string
  mood: string
  audio_url: string
  cover_url: string
  sort_order: number
  is_active: boolean
}
