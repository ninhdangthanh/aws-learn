interface IncomingCallProps {
  callerName: string
  onAccept: () => void
  onReject: () => void
}

function IncomingCall({ callerName, onAccept, onReject }: IncomingCallProps) {
  return (
    <div className="incoming-call-overlay">
      <div className="incoming-call-modal">
        <div className="incoming-call-pulse-ring" />
        <div className="incoming-call-avatar">
          {callerName.charAt(0).toUpperCase()}
        </div>
        <div className="incoming-call-info">
          <h3>{callerName}</h3>
          <p>Incoming video call…</p>
        </div>
        <div className="incoming-call-actions">
          <button
            className="incoming-call-btn incoming-call-btn--reject"
            onClick={onReject}
            title="Reject call"
          >
            <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
              <path d="M8.4 13.8C9.6 16 11.2 17.8 13.6 19L15.4 17.2C15.7 16.9 16.1 16.8 16.5 16.9C17.9 17.3 19.4 17.5 21 17.5C21.6 17.5 22 17.9 22 18.5V21.4C22 22 21.6 22.4 21 22.4C11.6 22.4 4 14.8 4 5.4C4 4.8 4.4 4.4 5 4.4H7.9C8.5 4.4 8.9 4.8 8.9 5.4C8.9 7 9.1 8.5 9.5 9.9C9.6 10.3 9.5 10.7 9.2 11L8.4 13.8Z" stroke="white" strokeWidth="2" />
              <line x1="4" y1="4" x2="24" y2="24" stroke="white" strokeWidth="2" strokeLinecap="round" />
            </svg>
          </button>
          <button
            className="incoming-call-btn incoming-call-btn--accept"
            onClick={onAccept}
            title="Accept call"
          >
            <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
              <path d="M8.4 13.8C9.6 16 11.2 17.8 13.6 19L15.4 17.2C15.7 16.9 16.1 16.8 16.5 16.9C17.9 17.3 19.4 17.5 21 17.5C21.6 17.5 22 17.9 22 18.5V21.4C22 22 21.6 22.4 21 22.4C11.6 22.4 4 14.8 4 5.4C4 4.8 4.4 4.4 5 4.4H7.9C8.5 4.4 8.9 4.8 8.9 5.4C8.9 7 9.1 8.5 9.5 9.9C9.6 10.3 9.5 10.7 9.2 11L8.4 13.8Z" stroke="white" strokeWidth="2" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  )
}

export default IncomingCall
