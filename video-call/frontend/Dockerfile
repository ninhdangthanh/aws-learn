# ---- Build Stage ----
FROM node:20-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

# ---- Serve Stage ----
FROM nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf.template
COPY env-config.template.js /etc/nginx/templates/env-config.template.js

EXPOSE 80

ENV BACKEND_URL=http://backend:8080
ENV API_URL=""
ENV WS_URL=""
ENV STUN_URLS=""
ENV TURN_URLS=""
ENV TURN_USERNAME=""
ENV TURN_CREDENTIAL=""
ENV ICE_TRANSPORT_POLICY="all"

CMD ["/bin/sh", "-c", "envsubst '${BACKEND_URL}' < /etc/nginx/conf.d/default.conf.template > /etc/nginx/conf.d/default.conf && envsubst '${API_URL} ${WS_URL} ${STUN_URLS} ${TURN_URLS} ${TURN_USERNAME} ${TURN_CREDENTIAL} ${ICE_TRANSPORT_POLICY}' < /etc/nginx/templates/env-config.template.js > /usr/share/nginx/html/env-config.js && nginx -g 'daemon off;'"]
