import { AuthProvider, useAuth } from './auth/AuthContext'
import { AuthPage } from './pages/AuthPage'
import { DashboardPage } from './pages/DashboardPage'

function Routes() {
  const { status } = useAuth()

  if (status === 'loading') {
    return (
      <div className="centered">
        <p className="muted">Đang khôi phục phiên đăng nhập…</p>
      </div>
    )
  }

  return status === 'authenticated' ? <DashboardPage /> : <AuthPage />
}

export default function App() {
  return (
    <AuthProvider>
      <Routes />
    </AuthProvider>
  )
}
