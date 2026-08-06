export interface User {
  id: string
  email: string
  full_name: string
  created_at: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: string
  /** Số giây còn lại của access token. */
  expires_in: number
  device_id: string
}

export interface AuthResponse {
  user: User
  tokens: TokenPair
}

export interface SessionInfo {
  jti: string
  device_id: string
  user_agent: string
  ip: string
  created_at: string
  expires_at: string
}

/** Lỗi có mã từ backend, ví dụ token_expired / invalid_credentials. */
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}
