const DEFAULT_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? ''

export class ApiError extends Error {
  readonly status: number
  readonly payload?: unknown

  constructor(message: string, status: number, payload?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
  }
}

export class UnauthorizedError extends ApiError {
  constructor(message = 'unauthorized', payload?: unknown) {
    super(message, 401, payload)
    this.name = 'UnauthorizedError'
  }
}

class ApiClient {
  private token: string | null
  private readonly baseUrl: string

  constructor(baseUrl = DEFAULT_BASE_URL) {
    this.baseUrl = baseUrl
    this.token = null
  }

  setToken(token: string | null) {
    this.token = token
  }

  async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const url = this.resolveUrl(path)
    const headers = new Headers(init.headers)
    const options: RequestInit = { ...init, headers }

    if (options.body && !(options.body instanceof FormData)) {
      if (!headers.has('Content-Type')) {
        headers.set('Content-Type', 'application/json')
      }
      if (typeof options.body !== 'string') {
        options.body = JSON.stringify(options.body)
      }
    }

    if (this.token && !headers.has('Authorization')) {
      headers.set('Authorization', `Bearer ${this.token}`)
    }

    const response = await fetch(url, options)

    let payload: unknown = undefined
    const contentType = response.headers.get('Content-Type') ?? ''
    if (contentType.includes('application/json')) {
      payload = await response.json().catch(() => undefined)
    } else {
      payload = await response.text().catch(() => undefined)
    }

    if (!response.ok) {
      if (response.status === 401) {
        throw new UnauthorizedError('unauthorized', payload)
      }
      const message = (payload as Record<string, unknown>)?.error
      throw new ApiError(
        typeof message === 'string' ? message : response.statusText,
        response.status,
        payload,
      )
    }

    return payload as T
  }

  get<T>(path: string, init?: RequestInit) {
    return this.request<T>(path, { ...init, method: 'GET' })
  }

  post<T>(path: string, body?: unknown, init?: RequestInit) {
    const options: RequestInit = { ...init, method: 'POST' }
    if (body !== undefined) {
      options.body = body as BodyInit
    }
    return this.request<T>(path, options)
  }

  put<T>(path: string, body?: unknown, init?: RequestInit) {
    const options: RequestInit = { ...init, method: 'PUT' }
    if (body !== undefined) {
      options.body = body as BodyInit
    }
    return this.request<T>(path, options)
  }

  delete(path: string, init?: RequestInit) {
    return this.request(path, { ...init, method: 'DELETE' })
  }

  private resolveUrl(path: string) {
    if (path.startsWith('http://') || path.startsWith('https://')) {
      return path
    }
    if (this.baseUrl) {
      return `${this.baseUrl.replace(/\/$/, '')}${path}`
    }
    return path
  }
}

export const apiClient = new ApiClient()
