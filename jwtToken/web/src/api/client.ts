import { ApiError, type AuthResponse, type SessionInfo, type TokenPair, type User } from './types'

const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080/api/v1'

const REFRESH_KEY = 'jwt-auth.refresh_token'
const DEVICE_KEY = 'jwt-auth.device_id'

/**
 * Access token chỉ nằm trong memory: reload trang là mất, nhưng đổi lại nó
 * không đọc được bằng XSS qua localStorage. Refresh token thì phải persist
 * để giữ đăng nhập, và nó thu hồi được ở server (Redis) nên rủi ro thấp hơn.
 */
let accessToken: string | null = null

/** Gọi khi refresh thất bại → UI cần đá user về màn login. */
let onSessionExpired: (() => void) | null = null

export function setSessionExpiredHandler(fn: (() => void) | null) {
  onSessionExpired = fn
}

export const tokenStore = {
  getAccessToken: () => accessToken,
  getRefreshToken: () => localStorage.getItem(REFRESH_KEY),

  /** Mỗi trình duyệt giữ một device_id cố định để server phân biệt thiết bị. */
  getDeviceId(): string {
    let id = localStorage.getItem(DEVICE_KEY)
    if (!id) {
      id = crypto.randomUUID()
      localStorage.setItem(DEVICE_KEY, id)
    }
    return id
  },

  save(tokens: TokenPair) {
    accessToken = tokens.access_token
    localStorage.setItem(REFRESH_KEY, tokens.refresh_token)
    if (tokens.device_id) localStorage.setItem(DEVICE_KEY, tokens.device_id)
  },

  clear() {
    accessToken = null
    localStorage.removeItem(REFRESH_KEY)
  },
}

interface RequestOptions {
  method?: string
  body?: unknown
  /** Có gắn Authorization header hay không (mặc định: không). */
  auth?: boolean
  /** Cờ nội bộ để chặn vòng lặp refresh vô hạn. */
  _retried?: boolean
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, auth = false, _retried = false } = opts

  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (auth && accessToken) headers['Authorization'] = `Bearer ${accessToken}`

  const res = await fetch(`${BASE_URL}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  if (res.ok) {
    return res.status === 204 ? (undefined as T) : ((await res.json()) as T)
  }

  const payload = await res.json().catch(() => ({}))
  const code: string = payload.error ?? 'unknown_error'
  const message: string = payload.message ?? `HTTP ${res.status}`

  // Access token hết hạn → refresh một lần rồi thử lại đúng request đó.
  // Đây là toàn bộ lý do access token có thể để TTL ngắn mà UX vẫn mượt.
  if (res.status === 401 && auth && !_retried && code === 'token_expired') {
    const refreshed = await refreshAccessToken()
    if (refreshed) {
      return request<T>(path, { ...opts, _retried: true })
    }
    onSessionExpired?.()
  }

  throw new ApiError(res.status, code, message)
}

/**
 * Gộp nhiều lời gọi refresh đồng thời vào một request duy nhất — nếu không,
 * 5 request cùng dính 401 sẽ bắn 5 lần /auth/refresh.
 */
let refreshInFlight: Promise<boolean> | null = null

function refreshAccessToken(): Promise<boolean> {
  if (refreshInFlight) return refreshInFlight

  refreshInFlight = (async () => {
    const refreshToken = tokenStore.getRefreshToken()
    if (!refreshToken) return false

    try {
      const res = await request<{ tokens: TokenPair }>('/auth/refresh', {
        method: 'POST',
        body: { refresh_token: refreshToken },
      })
      tokenStore.save(res.tokens)
      return true
    } catch {
      tokenStore.clear()
      return false
    } finally {
      refreshInFlight = null
    }
  })()

  return refreshInFlight
}

export const authApi = {
  register: (email: string, password: string, fullName: string) =>
    request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: { email, password, full_name: fullName },
    }),

  login: (email: string, password: string) =>
    request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: { email, password, device_id: tokenStore.getDeviceId() },
    }),

  refresh: refreshAccessToken,

  logout: () => {
    const refreshToken = tokenStore.getRefreshToken()
    if (!refreshToken) return Promise.resolve()
    return request<{ message: string }>('/auth/logout', {
      method: 'POST',
      body: { refresh_token: refreshToken },
    })
  },

  me: () => request<{ user: User }>('/me', { auth: true }),

  sessions: () => request<{ sessions: SessionInfo[] }>('/sessions', { auth: true }),
}
