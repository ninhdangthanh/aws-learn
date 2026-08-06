# Một Dockerfile dùng chung cho cả 4 service, chọn bằng build arg SERVICE.
# docker-compose.yml truyền SERVICE=gateway|order|inventory|notification.

FROM golang:1.25-alpine AS build

WORKDIR /src

# Copy manifest trước để lớp cache tải dependency không bị hỏng mỗi lần sửa code.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE
RUN test -n "$SERVICE" || (echo "cần truyền build arg SERVICE" && exit 1)
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/app ./cmd/${SERVICE}

FROM alpine:3.21

# ca-certificates cho HTTPS, curl để healthcheck và để bạn exec vào soi thủ công.
RUN apk add --no-cache ca-certificates curl tzdata

COPY --from=build /out/app /usr/local/bin/app

ENTRYPOINT ["/usr/local/bin/app"]
