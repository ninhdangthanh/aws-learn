import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

import { authApi, setSessionExpiredHandler, tokenStore } from '../api/client'
import type { User } from '../api/types'

type Status = 'loading' | 'authenticated' | 'anonymous'

interface AuthContextValue {
  user: User | null
  status: Status
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, fullName: string) => Promise<void>
  logout: () => Promise<void>
  logoutAll: () => Promise<void>
  changePassword: (currentPassword: string, newPassword: string) => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [status, setStatus] = useState<Status>('loading')

  // Khôi phục phiên sau khi reload: access token nằm trong memory nên đã mất,
  // phải đổi refresh token (localStorage) lấy access token mới rồi gọi /me.
  useEffect(() => {
    let cancelled = false

    async function bootstrap() {
      if (!tokenStore.getRefreshToken()) {
        if (!cancelled) setStatus('anonymous')
        return
      }

      const ok = await authApi.refresh()
      if (!ok) {
        if (!cancelled) setStatus('anonymous')
        return
      }

      try {
        const { user } = await authApi.me()
        if (cancelled) return
        setUser(user)
        setStatus('authenticated')
      } catch {
        if (cancelled) return
        tokenStore.clear()
        setStatus('anonymous')
      }
    }

    bootstrap()
    return () => {
      cancelled = true
    }
  }, [])

  // Refresh token cũng chết (hết 7 ngày / bị logout ở nơi khác) → về màn login.
  useEffect(() => {
    setSessionExpiredHandler(() => {
      tokenStore.clear()
      setUser(null)
      setStatus('anonymous')
    })
    return () => setSessionExpiredHandler(null)
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const res = await authApi.login(email, password)
    tokenStore.save(res.tokens)
    setUser(res.user)
    setStatus('authenticated')
  }, [])

  const register = useCallback(async (email: string, password: string, fullName: string) => {
    const res = await authApi.register(email, password, fullName)
    tokenStore.save(res.tokens)
    setUser(res.user)
    setStatus('authenticated')
  }, [])

  const logout = useCallback(async () => {
    try {
      await authApi.logout()
    } finally {
      // Dù server có lỗi thì client vẫn phải quên token đi.
      tokenStore.clear()
      setUser(null)
      setStatus('anonymous')
    }
  }, [])

  // Logout All huỷ luôn phiên của chính thiết bị này (token_version++ áp cho mọi token).
  const logoutAll = useCallback(async () => {
    try {
      await authApi.logoutAll()
    } finally {
      tokenStore.clear()
      setUser(null)
      setStatus('anonymous')
    }
  }, [])

  // Đổi mật khẩu thu hồi mọi phiên, nhưng server cấp lại token mới cho thiết bị
  // hiện tại nên user không bị tự đá ra ngoài.
  const changePassword = useCallback(async (currentPassword: string, newPassword: string) => {
    const res = await authApi.changePassword(currentPassword, newPassword)
    tokenStore.save(res.tokens)
    setUser(res.user)
  }, [])

  const value = useMemo(
    () => ({ user, status, login, register, logout, logoutAll, changePassword }),
    [user, status, login, register, logout, logoutAll, changePassword],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth phải được dùng bên trong <AuthProvider>')
  return ctx
}
