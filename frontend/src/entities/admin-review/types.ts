export interface AdminReview {
  id: string
  author_name: string
  rating: number
  body: string
  admin_reply: string
  admin_reply_at?: string
  is_published: boolean
  created_at: string
}
