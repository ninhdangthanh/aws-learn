const hostname = window.location.hostname
// If we're on the default Vite port, assume we need to point to the backend directly on 8080
// Otherwise (like in Docker/Production), we assume Nginx is proxying /api and /ws
const isDev = window.location.port === '5173'
const port = isDev ? ':8080' : ''

export const API_URL = `http://${hostname}${port}`
export const WS_URL = `ws://${hostname}${port}/ws`
