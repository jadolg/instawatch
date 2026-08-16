FROM golang:1.26.6-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o instawatch .

# Built from source with no --enable-lib* flags: downloader.go only ever
# stream-copies, so the external encoder libs the wolfi apk package pulls in
# (x264, x265, aom, srt, ssh, zeromq, ...) aren't needed.
FROM cgr.dev/chainguard/wolfi-base AS ffmpeg
RUN apk add --no-cache build-base nasm pkgconf wget xz zlib-dev

ARG FFMPEG_VERSION=7.1.1
# ffmpeg.org's own download server intermittently refuses/times out connections
# from GitHub Actions runners, which broke a release build. GitHub's mirror of
# the FFmpeg repo shares GitHub's infra and has been far more reliable in CI.
ARG FFMPEG_SHA256=f117507dc501f2a6c11f9241d8d0c3213846cfad91764361af37befd6b6c523d

RUN wget -q --tries=5 --timeout=30 --retry-connrefused -O /tmp/ffmpeg.tar.gz \
      "https://github.com/FFmpeg/FFmpeg/archive/refs/tags/n${FFMPEG_VERSION}.tar.gz" \
    && echo "${FFMPEG_SHA256}  /tmp/ffmpeg.tar.gz" | sha256sum -c - \
    && mkdir -p /tmp/src \
    && tar -xf /tmp/ffmpeg.tar.gz -C /tmp/src --strip-components=1

WORKDIR /tmp/src
RUN ./configure \
      --prefix=/opt/ffmpeg \
      --disable-static --enable-shared \
      --disable-doc --disable-debug \
      --disable-avdevice \
      --disable-network \
      --disable-ffplay \
      --disable-postproc \
    && make -j"$(nproc)" \
    && make install \
    && mkdir -p /data-empty

FROM cgr.dev/chainguard/python:latest-dev AS pydeps
USER root
WORKDIR /app
COPY requirements.txt .
RUN python -m venv /venv \
    && /venv/bin/pip install --no-cache-dir -r requirements.txt \
    && /venv/bin/python -m pip uninstall -y pip setuptools

FROM cgr.dev/chainguard/python:latest

COPY --from=ffmpeg /opt/ffmpeg/lib/*.so* /usr/lib/
COPY --from=ffmpeg /opt/ffmpeg/bin/ffmpeg /opt/ffmpeg/bin/ffprobe /usr/bin/

COPY --from=pydeps /venv /venv
ENV PATH="/venv/bin:${PATH}"

COPY --from=builder /app/instawatch /usr/local/bin/instawatch
COPY --from=ffmpeg --chown=nonroot:nonroot --chmod=700 /data-empty /data

ENV DATA_DIR=/data
VOLUME ["/data"]

EXPOSE 8080
ENTRYPOINT ["instawatch"]
