import { apiFetch } from '@shared/api'
import type { CreateReviewPayload, PublicReview, ReviewsResponse } from './types'

export const reviewApi = {
  list(page = 1, perPage = 50) {
    return apiFetch<ReviewsResponse>(`/reviews/?page=${page}&per_page=${perPage}`)
  },

  create(payload: CreateReviewPayload) {
    return apiFetch<PublicReview>('/reviews/', { method: 'POST', body: payload })
  },
}
