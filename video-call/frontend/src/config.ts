const runtimeConfig = window.__APP_CONFIG__ ?? {}
const hostname = window.location.hostname
const isDev = window.location.port === '5173'
const devApiOrigin = `http://${hostname}:8080`
const sameOriginApiUrl = window.location.origin
const sameOriginWsUrl = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/ws`

const splitUrls = (value?: string) =>
  value
    ?.split(',')
    .map((url) => url.trim())
    .filter(Boolean) ?? []

export const API_URL = runtimeConfig.API_URL || import.meta.env.VITE_API_URL || (isDev ? devApiOrigin : sameOriginApiUrl)
export const WS_URL = runtimeConfig.WS_URL || import.meta.env.VITE_WS_URL || (isDev ? `ws://${hostname}:8080/ws` : sameOriginWsUrl)

const stunUrls = splitUrls(runtimeConfig.STUN_URLS || import.meta.env.VITE_STUN_URLS)
const turnUrls = splitUrls(runtimeConfig.TURN_URLS || import.meta.env.VITE_TURN_URLS)
const turnUsername = runtimeConfig.TURN_USERNAME || import.meta.env.VITE_TURN_USERNAME
const turnCredential = runtimeConfig.TURN_CREDENTIAL || import.meta.env.VITE_TURN_CREDENTIAL

export const ICE_SERVERS: RTCIceServer[] = [
  ...(stunUrls.length > 0
    ? stunUrls.map((url) => ({ urls: url }))
    : [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun:stun1.l.google.com:19302' },
        { urls: 'stun:stun2.l.google.com:19302' },
      ]),
  ...(turnUrls.length > 0
    ? [
        {
          urls: turnUrls,
          username: turnUsername,
          credential: turnCredential,
        },
      ]
    : []),
]

export const WEBRTC_CONFIG: RTCConfiguration = {
  iceServers: ICE_SERVERS,
  iceTransportPolicy: 'all',
}
