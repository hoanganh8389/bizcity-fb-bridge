# [2026-11-24 Johnny Chu - Chu Hoàng Anh] PHASE-017 — dựng 2 binary cho Cell zalo-hub:
#   fbchat-bridge-e2ee : bridge upstream fbchat-v2, KHÔNG sửa (nhận / gửi Messenger, E2EE)
#   fbchat-login       : login/main.go của repo này (email + mật khẩu → cookie)
# SHA ghim: chỉ đổi sau khi spike S1–S7 PASS (PHASE-017 §8).
#
# Linux (image Cell):  docker build --output type=local,dest=dist .
# Windows (máy thử):   docker build --build-arg GOOS=windows --output type=local,dest=dist-win .
FROM golang:1.26-bookworm AS build
ARG FBCHAT_REPO=https://github.com/m008v/fbchat-v2.git
ARG FBCHAT_SHA=ec26e54c550d99aab31c666e5e2865794527bdf2
ARG META_SHA=cea2e38c9107be591bcd1b7a6562ccc72400ce23
ARG GOOS=linux
ARG GOARCH=amd64
ENV GOTOOLCHAIN=auto CGO_ENABLED=0
RUN git clone --quiet "$FBCHAT_REPO" /src \
 && cd /src && git checkout --quiet "$FBCHAT_SHA" \
 && git submodule update --init --quiet bridge-e2ee/meta \
 && test "$(git -C bridge-e2ee/meta rev-parse HEAD)" = "$META_SHA"
WORKDIR /src/bridge-e2ee
RUN go mod download
COPY login/ ./cmd/fbchat-login/
RUN EXT=$([ "$GOOS" = "windows" ] && echo .exe || true) \
 && GOOS=$GOOS GOARCH=$GOARCH go build -mod=mod -trimpath -ldflags "-s -w" -o /out/fbchat-bridge-e2ee$EXT . \
 && GOOS=$GOOS GOARCH=$GOARCH go build -mod=mod -trimpath -ldflags "-s -w" -o /out/fbchat-login$EXT ./cmd/fbchat-login \
 && printf 'fbchat-v2 %s\nmautrix-meta %s\n' "$FBCHAT_SHA" "$META_SHA" > /out/VERSIONS.txt

FROM scratch
COPY --from=build /out/ /
