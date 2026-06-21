import { API_BASE } from '@shared/config/env'

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly requestId?: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown
  accessToken?: string
}

export async function apiFetch<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, accessToken, ...rest } = options

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (accessToken) {
    headers['X-Access-Token'] = accessToken
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...rest,
    // Нужно для cookie-сессии админки (/admin/login) — без этого браузер не
    // отправит и не примет Set-Cookie для запросов через fetch.
    credentials: 'include',
    headers: { ...headers, ...(rest.headers as Record<string, string>) },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (!response.ok) {
    let message = `HTTP ${response.status}`
    let requestId: string | undefined
    try {
      const json = await response.json()
      message = json.error ?? message
      requestId = json.request_id
    } catch {
      // тело не JSON — используем дефолтное сообщение
    }
    throw new ApiError(response.status, message, requestId)
  }

  // 204 No Content — возвращаем пустой объект
  if (response.status === 204) return {} as T

  return response.json() as Promise<T>
}
