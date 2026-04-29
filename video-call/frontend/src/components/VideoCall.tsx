import { useEffect, useRef } from 'react'
import { CallStatus } from '../types'

interface VideoCallProps {
  localStream: MediaStream | null
  remoteStream: MediaStream | null
  callStatus: CallStatus
  endCall: () => void
  toggleAudio: () => void
  toggleVideo: () => void
  audioEnabled: boolean
  videoEnabled: boolean
}

function VideoCall({
  localStream,
  remoteStream,
  callStatus,
  endCall,
  toggleAudio,
  toggleVideo,
  audioEnabled,
  videoEnabled,
}: VideoCallProps) {
  const localVideoRef = useRef<HTMLVideoElement>(null)
  const remoteVideoRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    if (localVideoRef.current && localStream) {
      localVideoRef.current.srcObject = localStream
    }
  }, [localStream])

  useEffect(() => {
    if (remoteVideoRef.current && remoteStream) {
      remoteVideoRef.current.srcObject = remoteStream
    }
  }, [remoteStream])

  return (
    <div className="video-call-container">
      {/* Remote Video (full background) */}
      <div className="remote-video-wrapper">
        {remoteStream ? (
          <video
            ref={remoteVideoRef}
            autoPlay
            playsInline
            className="remote-video"
          />
        ) : (
          <div className="video-placeholder">
            <div className="video-placeholder-icon">
              <svg width="64" height="64" viewBox="0 0 64 64" fill="none">
                <circle cx="32" cy="24" r="12" stroke="rgba(255,255,255,0.4)" strokeWidth="2.5" />
                <path d="M10 54C10 43.507 20.059 35 32 35C43.941 35 54 43.507 54 54" stroke="rgba(255,255,255,0.4)" strokeWidth="2.5" strokeLinecap="round" />
              </svg>
            </div>
            <p>{callStatus === 'calling' ? 'Calling…' : 'Connecting…'}</p>
          </div>
        )}
      </div>

      {/* Local Video (picture-in-picture) */}
      <div className="local-video-wrapper">
        {localStream ? (
          <video
            ref={localVideoRef}
            autoPlay
            playsInline
            muted
            className="local-video"
          />
        ) : (
          <div className="local-video-placeholder">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="8" r="4" stroke="rgba(255,255,255,0.5)" strokeWidth="1.5" />
              <path d="M4 20C4 16.686 7.582 14 12 14C16.418 14 20 16.686 20 20" stroke="rgba(255,255,255,0.5)" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </div>
        )}
      </div>

      {/* Call Status */}
      {callStatus === 'calling' && (
        <div className="call-status-badge">
          <span className="call-status-pulse" />
          Calling…
        </div>
      )}

      {/* Controls */}
      <div className="call-controls">
        <button
          className={`control-btn ${!audioEnabled ? 'control-btn--off' : ''}`}
          onClick={toggleAudio}
          title={audioEnabled ? 'Mute microphone' : 'Unmute microphone'}
        >
          {audioEnabled ? (
            <svg width="22" height="22" viewBox="0 0 22 22" fill="none">
              <rect x="7" y="1" width="8" height="12" rx="4" stroke="currentColor" strokeWidth="1.8" />
              <path d="M3 11C3 15.418 6.582 19 11 19C15.418 19 19 15.418 19 11" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
              <line x1="11" y1="19" x2="11" y2="21" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
            </svg>
          ) : (
            <svg width="22" height="22" viewBox="0 0 22 22" fill="none">
              <rect x="7" y="1" width="8" height="12" rx="4" stroke="currentColor" strokeWidth="1.8" />
              <path d="M3 11C3 15.418 6.582 19 11 19C15.418 19 19 15.418 19 11" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
              <line x1="11" y1="19" x2="11" y2="21" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
              <line x1="2" y1="2" x2="20" y2="20" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
            </svg>
          )}
        </button>

        <button
          className="control-btn control-btn--end"
          onClick={endCall}
          title="End call"
        >
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
            <path d="M6.6 10.8C7.4 12.4 8.6 13.8 10.2 14.8L11.6 13.4C11.8 13.2 12.1 13.1 12.4 13.2C13.4 13.5 14.5 13.7 15.6 13.7C16.1 13.7 16.5 14.1 16.5 14.6V16.8C16.5 17.3 16.1 17.7 15.6 17.7C8.9 17.7 3.5 12.3 3.5 5.6C3.5 5.1 3.9 4.7 4.4 4.7H6.6C7.1 4.7 7.5 5.1 7.5 5.6C7.5 6.7 7.7 7.8 8 8.8C8.1 9.1 8 9.4 7.8 9.6L6.6 10.8Z" stroke="currentColor" strokeWidth="1.8" />
            <line x1="3" y1="3" x2="21" y2="21" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
          </svg>
        </button>

        <button
          className={`control-btn ${!videoEnabled ? 'control-btn--off' : ''}`}
          onClick={toggleVideo}
          title={videoEnabled ? 'Turn off camera' : 'Turn on camera'}
        >
          {videoEnabled ? (
            <svg width="22" height="22" viewBox="0 0 22 22" fill="none">
              <path d="M14 8.5L19.35 5.65C19.61 5.52 19.9 5.71 19.9 6V16C19.9 16.29 19.61 16.48 19.35 16.35L14 13.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
              <rect x="1.5" y="5" width="13" height="12" rx="2" stroke="currentColor" strokeWidth="1.8" />
            </svg>
          ) : (
            <svg width="22" height="22" viewBox="0 0 22 22" fill="none">
              <path d="M14 8.5L19.35 5.65C19.61 5.52 19.9 5.71 19.9 6V16C19.9 16.29 19.61 16.48 19.35 16.35L14 13.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
              <rect x="1.5" y="5" width="13" height="12" rx="2" stroke="currentColor" strokeWidth="1.8" />
              <line x1="1" y1="1" x2="21" y2="21" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
            </svg>
          )}
        </button>
      </div>
    </div>
  )
}

export default VideoCall
