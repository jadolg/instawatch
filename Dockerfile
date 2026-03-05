FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o instawatch .

FROM alpine:3.23

RUN apk add --no-cache python3 py3-pip ffmpeg \
    && pip3 install --break-system-packages yt-dlp

COPY --from=builder /app/instawatch /usr/local/bin/instawatch

RUN adduser -D -u 1000 instawatch \
    && mkdir -p /data \
    && chown instawatch:instawatch /data \
    && chmod 700 /data
ENV DATA_DIR=/data
VOLUME ["/data"]

USER instawatch
EXPOSE 8080
CMD ["instawatch"]
