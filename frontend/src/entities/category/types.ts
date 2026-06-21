// Ответ GET /api/v1/categories/
export interface Category {
  id: string
  title: string
  description: string
  cover_image_url: string
  seo_tags: string[]
  is_active: boolean
}

export type QuestionUIType = 'select' | 'tags' | 'radio' | 'textarea' | 'text'

export interface QuestionOption {
  value: string
  label: string
}

export interface Question {
  id: string
  question_text: string
  ui_type: QuestionUIType
  mapping_key: string
  is_required: boolean
  options: QuestionOption[]
}

// Ответ GET /api/v1/categories/:id/wizard
export interface WizardData {
  category_id: string
  questions: Question[]
}
