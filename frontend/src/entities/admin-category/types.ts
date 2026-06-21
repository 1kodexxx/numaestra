export interface AdminOption {
  label: string
  value: string
}

export interface AdminQuestion {
  id: number
  step_number: number
  question_text: string
  ui_type: 'text' | 'textarea' | 'select' | 'tags' | 'radio'
  mapping_key: string
  is_required: boolean
  options?: AdminOption[]
}

export interface AdminCategory {
  id: string
  title: string
  description: string
  cover_image_url: string
  seo_tags: string[]
  base_prompt_template: string
  questions?: AdminQuestion[]
}

export interface CategoryPayload {
  id?: string
  title: string
  description: string
  cover_image_url: string
  seo_tags: string[]
  base_prompt_template: string
}

export interface QuestionPayload {
  step_number: number
  question_text: string
  ui_type: AdminQuestion['ui_type']
  mapping_key: string
  is_required: boolean
  options: AdminOption[]
}
