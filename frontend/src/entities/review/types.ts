// Публичный отзыв о приложении (snake_case, как отдаёт бэкенд).
export interface PublicReview {
  id: string
  author_name: string
  rating: number
  body: string
  admin_reply?: string
  admin_reply_at?: string
  created_at: string
}

export interface ReviewsResponse {
  reviews: PublicReview[]
  total: number
}

export interface CreateReviewPayload {
  author_name: string
  rating: number
  body: string
}
