# [2026-11-24 Johnny Chu - Chu Hoàng Anh] PHASE-017 — dựng 2 binary cho Cell zalo-hub:
#   fbchat-bridge-e2ee : bridge upstream fbchat-v2, KHÔNG sửa (nhận / gửi Messenger, E2EE) — mautrix-meta ghim theo fbchat-v2 (META_SHA)
#   fbchat-login       : login/main.go của repo này (email + mật khẩu → cookie) — dựng trên mautrix-meta MỚI HƠN (META_LOGIN_SHA)
# Vì sao hai bản mautrix-meta: chạy thật 2026-11-24, bản cea2e38 vỡ ở bước điền mã email ("on_text_change: expected int64 in mins.ge,
# got float64"); upstream đã sửa ở 2d88a3bcd (2026-10-21) và còn nhiều bản sửa đăng nhập Messenger sau đó. Bridge nhận / gửi tin vẫn giữ bản
# fbchat-v2 đã thử; hai binary chỉ trao nhau cookie. Chỉ đổi SHA sau khi chạy thử PASS (PHASE-017 §8).
#
# Linux (image Cell):  docker build --output type=local,dest=dist .
# Windows (máy thử):   docker build --build-arg GOOS=windows --output type=local,dest=dist-win .
FROM golang:1.26-bookworm AS build
ARG FBCHAT_REPO=https://github.com/m008v/fbchat-v2.git
ARG FBCHAT_SHA=ec26e54c550d99aab31c666e5e2865794527bdf2
ARG META_SHA=cea2e38c9107be591bcd1b7a6562ccc72400ce23
ARG META_REPO=https://github.com/mautrix/meta.git
ARG META_LOGIN_SHA=6dbec4640d7e5a5c802ad1ea71410d4c0e4e586f
ARG GOOS=linux
ARG GOARCH=amd64
ENV GOTOOLCHAIN=auto CGO_ENABLED=0
RUN git clone --quiet "$FBCHAT_REPO" /src \
 && cd /src && git checkout --quiet "$FBCHAT_SHA" \
 && git submodule update --init --quiet bridge-e2ee/meta \
 && test "$(git -C bridge-e2ee/meta rev-parse HEAD)" = "$META_SHA" \
 && git clone --quiet "$META_REPO" /meta-login && git -C /meta-login checkout --quiet "$META_LOGIN_SHA"
WORKDIR /src/bridge-e2ee
RUN go mod download
RUN EXT=$([ "$GOOS" = "windows" ] && echo .exe || true) \
 && GOOS=$GOOS GOARCH=$GOARCH go build -mod=mod -trimpath -ldflags "-s -w" -o /out/fbchat-bridge-e2ee$EXT .
# fbchat-login dựng NGAY TRONG module mautrix-meta (import go.mau.fi/mautrix-meta/pkg/... tự khớp)
WORKDIR /meta-login
COPY login/ ./cmd/fbchat-login/
RUN EXT=$([ "$GOOS" = "windows" ] && echo .exe || true) \
 && GOOS=$GOOS GOARCH=$GOARCH go build -mod=mod -trimpath -ldflags "-s -w" -o /out/fbchat-login$EXT ./cmd/fbchat-login \
 && printf 'fbchat-v2 %s\nmautrix-meta (bridge) %s\nmautrix-meta (login) %s\n' "$FBCHAT_SHA" "$META_SHA" "$META_LOGIN_SHA" > /out/VERSIONS.txt

FROM scratch
COPY --from=build /out/ /
