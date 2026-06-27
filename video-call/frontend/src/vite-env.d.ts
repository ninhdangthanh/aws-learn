/// <reference types="vite/client" />

interface Window {
  __APP_CONFIG__?: {
    API_URL?: string
    WS_URL?: string
    STUN_URLS?: string
    TURN_URLS?: string
    TURN_USERNAME?: string
    TURN_CREDENTIAL?: string
  }
}
