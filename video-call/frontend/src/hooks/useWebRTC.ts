import { useRef, useState, useCallback, useEffect } from 'react'
import { SignalMessage, CallStatus } from '../types'

// ICE servers for NAT traversal - WebRTC uses UDP for media transport
const ICE_SERVERS: RTCConfiguration = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
    { urls: 'stun:stun2.l.google.com:19302' },
  ],
  // Prefer UDP transport for lower latency video calls
  iceTransportPolicy: 'all',
}

interface UseWebRTCReturn {
  localStream: MediaStream | null
  remoteStream: MediaStream | null
  callStatus: CallStatus
  incomingCall: { from: number; fromName: string } | null
  startCall: (targetUserId: number) => Promise<void>
  acceptCall: () => Promise<void>
  rejectCall: () => void
  endCall: () => void
  toggleAudio: () => void
  toggleVideo: () => void
  audioEnabled: boolean
  videoEnabled: boolean
}

export function useWebRTC(
  userId: number,
  sendMessage: (msg: SignalMessage) => void,
  lastMessage: SignalMessage | null
): UseWebRTCReturn {
  const peerConnectionRef = useRef<RTCPeerConnection | null>(null)
  const localStreamRef = useRef<MediaStream | null>(null)
  const remoteStreamRef = useRef<MediaStream | null>(null)

  const [localStream, setLocalStream] = useState<MediaStream | null>(null)
  const [remoteStream, setRemoteStream] = useState<MediaStream | null>(null)
  const [callStatus, setCallStatus] = useState<CallStatus>('idle')
  const [incomingCall, setIncomingCall] = useState<{ from: number; fromName: string } | null>(null)
  const [audioEnabled, setAudioEnabled] = useState(true)
  const [videoEnabled, setVideoEnabled] = useState(true)

  const pendingCandidatesRef = useRef<RTCIceCandidateInit[]>([])
  const targetUserRef = useRef<number>(0)

  const getMediaStream = useCallback(async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: {
          width: { ideal: 1280 },
          height: { ideal: 720 },
          facingMode: 'user',
        },
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      })
      localStreamRef.current = stream
      setLocalStream(stream)
      return stream
    } catch (err) {
      console.error('Failed to get media stream:', err)
      throw err
    }
  }, [])

  const createPeerConnection = useCallback((targetUserId: number) => {
    const pc = new RTCPeerConnection(ICE_SERVERS)

    // Add local tracks to the peer connection
    if (localStreamRef.current) {
      localStreamRef.current.getTracks().forEach((track) => {
        pc.addTrack(track, localStreamRef.current!)
      })
    }

    // Handle remote tracks - this is where we receive the remote video/audio over UDP
    const remoteStream = new MediaStream()
    remoteStreamRef.current = remoteStream
    setRemoteStream(remoteStream)

    pc.ontrack = (event) => {
      event.streams[0].getTracks().forEach((track) => {
        remoteStream.addTrack(track)
      })
      setRemoteStream(new MediaStream(remoteStream.getTracks()))
    }

    // Handle ICE candidates (UDP connection establishment)
    pc.onicecandidate = (event) => {
      if (event.candidate) {
        sendMessage({
          type: 'ice-candidate',
          from: userId,
          to: targetUserId,
          payload: event.candidate.toJSON(),
          fromName: '',
        })
      }
    }

    pc.oniceconnectionstatechange = () => {
      console.log('ICE connection state:', pc.iceConnectionState)
      if (pc.iceConnectionState === 'connected') {
        setCallStatus('connected')
      } else if (pc.iceConnectionState === 'disconnected' || pc.iceConnectionState === 'failed') {
        endCall()
      }
    }

    peerConnectionRef.current = pc
    return pc
  }, [userId, sendMessage])

  const startCall = useCallback(async (targetUserId: number) => {
    targetUserRef.current = targetUserId
    setCallStatus('calling')

    await getMediaStream()
    const pc = createPeerConnection(targetUserId)

    // Send call notification
    sendMessage({
      type: 'call',
      from: userId,
      to: targetUserId,
      payload: null,
      fromName: '',
    })

    // Create and send offer with SDP (Session Description Protocol)
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)

    sendMessage({
      type: 'offer',
      from: userId,
      to: targetUserId,
      payload: offer,
      fromName: '',
    })
  }, [userId, sendMessage, getMediaStream, createPeerConnection])

  const acceptCall = useCallback(async () => {
    if (!incomingCall) return

    targetUserRef.current = incomingCall.from
    setCallStatus('connected')

    await getMediaStream()
    const pc = createPeerConnection(incomingCall.from)

    // Process any pending ICE candidates
    for (const candidate of pendingCandidatesRef.current) {
      await pc.addIceCandidate(new RTCIceCandidate(candidate))
    }
    pendingCandidatesRef.current = []

    sendMessage({
      type: 'call-accepted',
      from: userId,
      to: incomingCall.from,
      payload: null,
      fromName: '',
    })

    setIncomingCall(null)
  }, [incomingCall, userId, sendMessage, getMediaStream, createPeerConnection])

  const rejectCall = useCallback(() => {
    if (!incomingCall) return

    sendMessage({
      type: 'call-rejected',
      from: userId,
      to: incomingCall.from,
      payload: null,
      fromName: '',
    })

    setIncomingCall(null)
    setCallStatus('idle')
    pendingCandidatesRef.current = []
  }, [incomingCall, userId, sendMessage])

  const endCall = useCallback(() => {
    if (targetUserRef.current) {
      sendMessage({
        type: 'call-ended',
        from: userId,
        to: targetUserRef.current,
        payload: null,
        fromName: '',
      })
    }

    // Cleanup
    if (peerConnectionRef.current) {
      peerConnectionRef.current.close()
      peerConnectionRef.current = null
    }

    if (localStreamRef.current) {
      localStreamRef.current.getTracks().forEach((track) => track.stop())
      localStreamRef.current = null
    }

    remoteStreamRef.current = null
    setLocalStream(null)
    setRemoteStream(null)
    setCallStatus('idle')
    setIncomingCall(null)
    targetUserRef.current = 0
    pendingCandidatesRef.current = []
    setAudioEnabled(true)
    setVideoEnabled(true)
  }, [userId, sendMessage])

  const toggleAudio = useCallback(() => {
    if (localStreamRef.current) {
      const audioTrack = localStreamRef.current.getAudioTracks()[0]
      if (audioTrack) {
        audioTrack.enabled = !audioTrack.enabled
        setAudioEnabled(audioTrack.enabled)
      }
    }
  }, [])

  const toggleVideo = useCallback(() => {
    if (localStreamRef.current) {
      const videoTrack = localStreamRef.current.getVideoTracks()[0]
      if (videoTrack) {
        videoTrack.enabled = !videoTrack.enabled
        setVideoEnabled(videoTrack.enabled)
      }
    }
  }, [])

  // Handle incoming signaling messages
  useEffect(() => {
    if (!lastMessage) return

    const handleMessage = async () => {
      switch (lastMessage.type) {
        case 'call':
          setIncomingCall({ from: lastMessage.from, fromName: lastMessage.fromName })
          setCallStatus('receiving')
          break

        case 'call-accepted': {
          setCallStatus('connected')
          break
        }

        case 'call-rejected':
          endCall()
          break

        case 'call-ended':
          endCall()
          break

        case 'offer': {
          const pc = peerConnectionRef.current
          if (pc) {
            await pc.setRemoteDescription(new RTCSessionDescription(lastMessage.payload))
            const answer = await pc.createAnswer()
            await pc.setLocalDescription(answer)

            sendMessage({
              type: 'answer',
              from: userId,
              to: lastMessage.from,
              payload: answer,
              fromName: '',
            })
          }
          break
        }

        case 'answer': {
          const pc = peerConnectionRef.current
          if (pc) {
            await pc.setRemoteDescription(new RTCSessionDescription(lastMessage.payload))
          }
          break
        }

        case 'ice-candidate': {
          const pc = peerConnectionRef.current
          if (pc && pc.remoteDescription) {
            await pc.addIceCandidate(new RTCIceCandidate(lastMessage.payload))
          } else {
            pendingCandidatesRef.current.push(lastMessage.payload)
          }
          break
        }
      }
    }

    handleMessage().catch(console.error)
  }, [lastMessage, userId, sendMessage, endCall])

  return {
    localStream,
    remoteStream,
    callStatus,
    incomingCall,
    startCall,
    acceptCall,
    rejectCall,
    endCall,
    toggleAudio,
    toggleVideo,
    audioEnabled,
    videoEnabled,
  }
}
