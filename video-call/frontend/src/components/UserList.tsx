import { OnlineUser, CallStatus } from '../types'

interface UserListProps {
  users: OnlineUser[]
  onCallUser: (userId: number) => void
  callStatus: CallStatus
  currentUserId: number
}

function UserList({ users, onCallUser, callStatus }: UserListProps) {
  if (users.length === 0) {
    return (
      <div className="user-list-empty">
        <p>No other users online</p>
        <span>Share the app with friends to start calling</span>
      </div>
    )
  }

  return (
    <div className="user-list">
      {users.map((u) => (
        <div key={u.id} className="user-list-item">
          <div className="user-list-avatar">
            {u.display_name.charAt(0).toUpperCase()}
            <span className="online-indicator" />
          </div>
          <div className="user-list-info">
            <span className="user-list-name">{u.display_name}</span>
            <span className="user-list-username">@{u.username}</span>
          </div>
          <button
            className="call-btn"
            onClick={() => onCallUser(u.id)}
            disabled={callStatus !== 'idle'}
            title={`Call ${u.display_name}`}
          >
            <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
              <path d="M11.25 6.75L14.82 4.65C15.03 4.52 15.3 4.66 15.3 4.91V13.09C15.3 13.34 15.03 13.48 14.82 13.35L11.25 11.25" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              <rect x="2.25" y="4.5" width="9.75" height="9" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          </button>
        </div>
      ))}
    </div>
  )
}

export default UserList
