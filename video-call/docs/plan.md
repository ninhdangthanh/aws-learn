# 📹 Video Call App (ReactJS + Golang) — Implementation Plan

## 1. 🎯 Goal

Build a simple video calling web application with:

* User authentication (sign up / login)
* Online user list
* 1-1 video call between users
* Real-time signaling using WebSocket
* Peer-to-peer media using WebRTC

---

## 2. 🧱 Tech Stack

### Frontend

* ReactJS (Vite or CRA)
* WebRTC API (browser native)
* WebSocket (native or socket.io-client)

### Backend

* Golang
* Gin (HTTP framework)
* Gorilla WebSocket
* JWT (authentication)

### Infrastructure (optional for MVP)

* STUN server (Google public)
* TURN server (optional, production)

---

## 3. 🏗️ System Architecture

* Frontend connects to backend via REST + WebSocket
* Backend handles:

  * Authentication
  * User presence
  * Signaling messages
* WebRTC handles:

  * Audio/video streaming (P2P via UDP)

```
[React Client A] ←→ [WebSocket Server (Go)] ←→ [React Client B]
       ↓                                          ↓
   WebRTC (P2P connection via ICE/STUN/TURN)
```

---

## 4. 🔐 Authentication Flow

### API Endpoints

* POST /api/register
* POST /api/login

### JWT Flow

1. User logs in
2. Server returns JWT
3. Client stores JWT (localStorage)
4. Client sends JWT in:

   * REST requests (Authorization header)
   * WebSocket connection (query or header)

---

## 5. 🧑‍🤝‍🧑 User Presence System

Backend maintains:

```go
map[userID]*WebSocketConnection
```

Events:

* user_online
* user_offline
* get_online_users

---

## 6. 📡 WebSocket Signaling Design

### Message Types

```json
{
  "type": "call_user",
  "targetUserId": "123",
  "from": "456"
}
```

```json
{
  "type": "offer",
  "sdp": "...",
  "targetUserId": "123"
}
```

```json
{
  "type": "answer",
  "sdp": "...",
  "targetUserId": "123"
}
```

```json
{
  "type": "ice_candidate",
  "candidate": "...",
  "targetUserId": "123"
}
```

```json
{
  "type": "end_call"
}
```

---

## 7. 🔁 Call Flow

### Step-by-step

1. User A clicks "Call"

2. Send `call_user` → server → User B

3. User B:

   * Accept → start WebRTC

4. A:

   * createOffer()
   * setLocalDescription()
   * send `offer`

5. B:

   * setRemoteDescription(offer)
   * createAnswer()
   * send `answer`

6. A:

   * setRemoteDescription(answer)

7. Both:

   * exchange ICE candidates

8. Connection established → media flows P2P

---

## 8. 🎥 WebRTC Setup (Frontend)

### Create Peer Connection

```js
const pc = new RTCPeerConnection({
  iceServers: [
    { urls: "stun:stun.l.google.com:19302" }
  ]
});
```

### Get Media

```js
const stream = await navigator.mediaDevices.getUserMedia({
  video: true,
  audio: true
});
```

### Add Tracks

```js
stream.getTracks().forEach(track => pc.addTrack(track, stream));
```

---

## 9. 🧠 ICE Candidate Handling

```js
pc.onicecandidate = (event) => {
  if (event.candidate) {
    send({
      type: "ice_candidate",
      candidate: event.candidate
    });
  }
};
```

---

## 10. 🖥️ UI Pages

### Pages

* Login / Register
* Dashboard:

  * Online users list
  * Call button
* Call screen:

  * Local video
  * Remote video
  * End call button

---

## 11. 🧪 MVP Scope (Phase 1)

* Login / Register
* WebSocket signaling
* 1-1 video call
* Online user list

---

## 12. 🚀 Phase 2 (Enhancements)

* TURN server integration
* Reconnect logic
* Call timeout
* Mute mic / turn off camera
* Call history

---

## 13. ⚠️ Important Notes

* Do NOT send video through backend
* Use WebRTC for media only
* Backend is only for signaling
* Always secure WebSocket (JWT)

---

## 14. 🧩 Suggested Folder Structure

### Frontend

```
src/
  components/
  pages/
  services/
    websocket.js
    webrtc.js
  hooks/
```

### Backend

```
/cmd
/internal
  /auth
  /ws
  /user
  /models
```

---

## 15. ✅ Definition of Done

* User can login
* See online users
* Click and call another user
* Video/audio works both ways
* Call can be ended cleanly

---

## 16. 🧠 Bonus (Optional)

* Use UUID for session/call ID
* Add simple logging for debugging WebRTC
* Add reconnect handling for WebSocket

---

END OF PLAN
