import { useCallback, useEffect, useState } from 'react'

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
  const { user, logout } = useAuth()
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

  async function callMe() {
    try {
      const res = await authApi.me()
      appendLog(`GET /me → 200 · ${res.user.email}`, true)
    } catch (err) {
      appendLog(`GET /me → ${err instanceof ApiError ? `${err.status} ${err.code}` : 'lỗi mạng'}`, false)
    }
  }

  async function forceRefresh() {
    const ok = await authApi.refresh()
    appendLog(ok ? 'POST /auth/refresh → access token mới' : 'POST /auth/refresh → thất bại', ok)
    if (ok) setNow(Math.floor(Date.now() / 1000))
  }

  return (
    <div className="page">
      <header className="topbar">
        <div>
          <strong>{user?.full_name || user?.email}</strong>
          <div className="muted small">{user?.email}</div>
        </div>
        <button className="ghost" onClick={() => void logout()}>
          Đăng xuất
        </button>
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
              <dd>{claims.ver}</dd>
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
            <button className="ghost" onClick={() => void forceRefresh()}>
              Refresh thủ công
            </button>
            <button className="ghost" onClick={() => void loadSessions()}>
              Tải lại sessions
            </button>
          </div>
        </section>

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
