import { useCallback, useEffect, useState, type FormEvent } from 'react'

import { authApi, tokenStore } from '../api/client'
import { ApiError, type SessionInfo } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { decodeJwt, formatCountdown } from '../utils/jwt'

interface LogEntry {
  id: number
  time: string
  text: string
  ok: boolean
}

let logSeq = 0

export function DashboardPage() {
  const { user, logout, logoutAll, changePassword } = useAuth()
  const [sessions, setSessions] = useState<SessionInfo[]>([])
  const [logs, setLogs] = useState<LogEntry[]>([])
  // Tick mỗi giây để đếm ngược hạn của access token đang giữ trong memory.
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000))

  const appendLog = useCallback((text: string, ok: boolean) => {
    setLogs((prev) =>
      [{ id: ++logSeq, time: new Date().toLocaleTimeString(), text, ok }, ...prev].slice(0, 12),
    )
  }, [])

  useEffect(() => {
    const timer = setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000)
    return () => clearInterval(timer)
  }, [])

  const loadSessions = useCallback(async () => {
    try {
      const res = await authApi.sessions()
      setSessions(res.sessions)
    } catch (err) {
      appendLog(err instanceof ApiError ? err.message : 'Lỗi tải sessions', false)
    }
  }, [appendLog])

  useEffect(() => {
    void loadSessions()
  }, [loadSessions])

  const accessToken = tokenStore.getAccessToken()
  const claims = accessToken ? decodeJwt(accessToken) : null
  const secondsLeft = claims ? claims.exp - now : 0

  // Rotation: mỗi lần refresh, jti của refresh token phải đổi sang giá trị mới.
  const refreshToken = tokenStore.getRefreshToken()
  const refreshClaims = refreshToken ? decodeJwt(refreshToken) : null

  async function callMe() {
    try {
      const res = await authApi.me()
      appendLog(`GET /me → 200 · ${res.user.email}`, true)
    } catch (err) {
      appendLog(`GET /me → ${err instanceof ApiError ? `${err.status} ${err.code}` : 'lỗi mạng'}`, false)
    }
  }

  async function forceRefresh() {
    const oldJti = refreshClaims?.jti.slice(0, 8) ?? '—'
    const ok = await authApi.refresh()
    if (ok) {
      const next = tokenStore.getRefreshToken()
      const newJti = (next ? decodeJwt(next)?.jti : null)?.slice(0, 8) ?? '—'
      appendLog(`POST /auth/refresh → rotate refresh ${oldJti} → ${newJti}`, true)
      setNow(Math.floor(Date.now() / 1000))
      void loadSessions()
    } else {
      appendLog('POST /auth/refresh → 401, refresh token đã bị thu hồi', false)
    }
  }

  // Dùng lại refresh token cũ sau khi đã rotate: minh hoạ Bài 3, phải nhận 401.
  async function replayOldRefresh() {
    const stale = refreshToken
    if (!stale) return

    const ok = await authApi.refresh()
    if (!ok) {
      appendLog('Không rotate được để lấy token cũ', false)
      return
    }

    const res = await fetch(`${import.meta.env.VITE_API_URL ?? 'http://localhost:8080/api/v1'}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: stale }),
    })
    appendLog(`Dùng lại refresh token cũ → ${res.status} ${res.ok ? '(SAI!)' : '(đúng, đã bị thu hồi)'}`, !res.ok)
    setNow(Math.floor(Date.now() / 1000))
  }

  return (
    <div className="page">
      <header className="topbar">
        <div>
          <strong>{user?.full_name || user?.email}</strong>
          <div className="muted small">{user?.email}</div>
        </div>
        <div className="actions">
          <button className="ghost" onClick={() => void logout()}>
            Đăng xuất
          </button>
          <button className="danger" onClick={() => void logoutAll()}>
            Đăng xuất tất cả thiết bị
          </button>
        </div>
      </header>

      <div className="grid">
        <section className="card">
          <h2>Access token</h2>
          <p className="muted small">
            Giữ trong memory, hết hạn sau {claims ? formatCountdown(secondsLeft) : '—'}. Khi hết hạn,
            request kế tiếp sẽ tự gọi <code>/auth/refresh</code> rồi retry.
          </p>
          {claims ? (
            <dl className="kv">
              <dt>sub</dt>
              <dd className="mono">{claims.sub}</dd>
              <dt>jti</dt>
              <dd className="mono">{claims.jti}</dd>
              <dt>typ</dt>
              <dd>{claims.typ}</dd>
              <dt>ver</dt>
              <dd>
                {claims.ver} <span className="muted small">so với token_version trong DB</span>
              </dd>
              <dt>exp</dt>
              <dd>
                {new Date(claims.exp * 1000).toLocaleTimeString()}{' '}
                <span className={secondsLeft > 0 ? 'badge ok' : 'badge warn'}>
                  {formatCountdown(secondsLeft)}
                </span>
              </dd>
            </dl>
          ) : (
            <p className="muted">Chưa có access token trong memory.</p>
          )}

          <div className="actions">
            <button className="primary" onClick={() => void callMe()}>
              Gọi GET /me
            </button>
            <button className="ghost" onClick={() => void loadSessions()}>
              Tải lại sessions
            </button>
          </div>
        </section>

        <section className="card">
          <h2>Refresh token (rotation)</h2>
          <p className="muted small">
            Mỗi lần refresh, jti cũ bị <code>DEL</code> khỏi Redis và thay bằng jti mới. Token cũ
            dùng lại sẽ nhận 401.
          </p>
          {refreshClaims ? (
            <dl className="kv">
              <dt>jti</dt>
              <dd className="mono">{refreshClaims.jti}</dd>
              <dt>device</dt>
              <dd className="mono">{refreshClaims.device_id ?? '—'}</dd>
              <dt>exp</dt>
              <dd>{new Date(refreshClaims.exp * 1000).toLocaleString()}</dd>
            </dl>
          ) : (
            <p className="muted">Không có refresh token.</p>
          )}
          <div className="actions">
            <button className="ghost" onClick={() => void forceRefresh()}>
              Rotate ngay
            </button>
            <button className="ghost" onClick={() => void replayOldRefresh()}>
              Thử dùng lại token cũ
            </button>
          </div>
        </section>

        <ChangePasswordCard onSubmit={changePassword} onLog={appendLog} />

        <section className="card">
          <h2>Thiết bị đang đăng nhập ({sessions.length})</h2>
          <p className="muted small">
            Mỗi refresh token là một phiên, lưu ở Redis với TTL = hạn của token.
          </p>
          {sessions.length === 0 ? (
            <p className="muted">Chưa có phiên nào.</p>
          ) : (
            <ul className="list">
              {sessions.map((s) => (
                <li key={s.jti}>
                  <div className="row">
                    <span className="mono">{s.device_id.slice(0, 18)}</span>
                    {s.device_id === tokenStore.getDeviceId() && (
                      <span className="badge ok">thiết bị này</span>
                    )}
                  </div>
                  <div className="muted small">
                    {s.user_agent.slice(0, 60)} · {s.ip}
                  </div>
                  <div className="muted small">
                    Hết hạn: {new Date(s.expires_at).toLocaleString()}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="card">
          <h2>Nhật ký request</h2>
          {logs.length === 0 ? (
            <p className="muted">Bấm một nút bên trái để xem kết quả.</p>
          ) : (
            <ul className="logs">
              {logs.map((l) => (
                <li key={l.id} className={l.ok ? 'ok' : 'err'}>
                  <span className="muted small">{l.time}</span> {l.text}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </div>
  )
}

interface ChangePasswordCardProps {
  onSubmit: (currentPassword: string, newPassword: string) => Promise<void>
  onLog: (text: string, ok: boolean) => void
}

function ChangePasswordCard({ onSubmit, onLog }: ChangePasswordCardProps) {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [busy, setBusy] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await onSubmit(current, next)
      setCurrent('')
      setNext('')
      onLog('Đổi mật khẩu thành công · token_version++ · cấp token mới', true)
    } catch (err) {
      onLog(err instanceof ApiError ? `Đổi mật khẩu: ${err.message}` : 'Lỗi mạng', false)
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="card">
      <h2>Đổi mật khẩu</h2>
      <p className="muted small">
        Tăng <code>token_version</code> nên mọi phiên khác bị đá ra ngay; thiết bị này được cấp
        token mới.
      </p>
      <form onSubmit={handleSubmit}>
        <label>
          Mật khẩu hiện tại
          <input
            type="password"
            required
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
            autoComplete="current-password"
          />
        </label>
        <label>
          Mật khẩu mới
          <input
            type="password"
            required
            minLength={8}
            value={next}
            onChange={(e) => setNext(e.target.value)}
            autoComplete="new-password"
          />
        </label>
        <button type="submit" className="primary" disabled={busy}>
          {busy ? 'Đang đổi…' : 'Đổi mật khẩu'}
        </button>
      </form>
    </section>
  )
}
