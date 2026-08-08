FROM golang:1.26.5-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o instawatch .

# ffmpeg has no runtime-only Chainguard image on the free tier, and the wolfi apk
# package pulls in ~190MB of encoder libraries (x264/x265/aom/svt-av1/opus/...)
# that this app never uses: downloader.go only ever stream-copies ("-c copy"),
# it never transcodes. So build a minimal ffmpeg from source instead, with no
# --enable-lib* flags, which keeps every native demuxer/muxer/decoder (plenty
# for remuxing whatever Instagram/Facebook serve) but drops all those optional
# encoder dependencies. --disable-network is safe (ffmpeg only ever touches
# local temp files here) and shrinks the attack surface further.
FROM cgr.dev/chainguard/wolfi-base AS ffmpeg
RUN apk add --no-cache build-base nasm pkgconf wget xz zlib-dev

ARG FFMPEG_VERSION=7.1.1
# Self-pinned from the official https://ffmpeg.org/releases/ download (no
# published sha256sum file upstream, only a GPG .asc signature) to catch
# corruption/tampering on rebuilds.
ARG FFMPEG_SHA256=733984395e0dbbe5c046abda2dc49a5544e7e0e1e2366bba849222ae9e3a03b1

RUN wget -q -O /tmp/ffmpeg.tar.xz "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz" \
    && echo "${FFMPEG_SHA256}  /tmp/ffmpeg.tar.xz" | sha256sum -c - \
    && mkdir -p /tmp/src \
    && tar -xf /tmp/ffmpeg.tar.xz -C /tmp/src --strip-components=1

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
