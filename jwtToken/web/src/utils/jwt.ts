export interface DecodedClaims {
  sub: string
  jti: string
  email?: string
  typ: string
  ver: number
  device_id?: string
  iat: number
  exp: number
  iss: string
}

/**
 * Decode payload của JWT để hiển thị. KHÔNG verify chữ ký — client không giữ
 * secret và cũng không được tin payload này; verify là việc của server.
 */
export function decodeJwt(token: string): DecodedClaims | null {
  const parts = token.split('.')
  if (parts.length !== 3) return null

  try {
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), '=')
    // decodeURIComponent(escape(...)) để đọc đúng ký tự non-ASCII trong payload.
    const json = decodeURIComponent(
      atob(padded)
        .split('')
        .map((c) => '%' + c.charCodeAt(0).toString(16).padStart(2, '0'))
        .join(''),
    )
    return JSON.parse(json) as DecodedClaims
  } catch {
    return null
  }
}

/** Format số giây còn lại thành "12m 34s"; <= 0 nghĩa là đã hết hạn. */
export function formatCountdown(seconds: number): string {
  if (seconds <= 0) return 'đã hết hạn'
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return m > 0 ? `${m}m ${s}s` : `${s}s`
}
