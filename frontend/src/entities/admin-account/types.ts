export type AccountStatus = 'active' | 'busy' | 'cooldown' | 'out_of_tokens' | 'banned'

export interface AdminAccount {
  id: string
  email: string
  status: AccountStatus
  token_balance: number
  max_concurrent_tasks: number
  concurrent_tasks: number
  updated_at: string
}

export interface AddAccountPayload {
  email: string
  session: string
  max_concurrent: number
}
