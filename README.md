# bizcity-fb-bridge

Binary Facebook cá nhân cho Cell zalo-hub (PHASE-017 của Open CRM). Cell gọi các binary này làm tiến trình con, qua JSON-RPC theo dòng trên stdin/stdout. Mã Cell không nằm ở đây.

| Binary | Nguồn | Việc |
|---|---|---|
| `fbchat-bridge-e2ee` | [fbchat-v2](https://github.com/m008v/fbchat-v2) `bridge-e2ee/`, ghim SHA, **không sửa** | Kết nối Messenger, nhận / gửi tin thường và E2EE |
| `fbchat-login` | `login/main.go` của repo này | Đăng nhập bằng email + mật khẩu (luồng Messenger Lite của mautrix-meta), trả cookie cho Cell |

## Dựng

```sh
docker build --output type=local,dest=dist .                              # Linux, cho image Cell
docker build --build-arg GOOS=windows --output type=local,dest=dist-win . # Windows, cho máy thử
```

## Giấy phép

- `fbchat-v2`: Apache-2.0. Phần `bridge/` của nó chép từ meta-messenger.js: AGPL-3.0.
- `mautrix-meta`: AGPL-3.0.
- Binary dựng ra liên kết với mã AGPL-3.0, nên repo này (kể cả `login/`) phát hành theo **AGPL-3.0** và để công khai (quyết định Q017-3, 2026-11-24). Cell zalo-hub chỉ chạy binary này như tiến trình con riêng, qua stdio.

## Luật dữ liệu

- Mật khẩu và mã 2FA chỉ nằm trong bộ nhớ của `fbchat-login`. Không ghi đĩa, không ghi log.
- Cookie và trạng thái thiết bị E2EE chỉ đi về tiến trình cha. Cell mã hoá chúng (`*.enc`).
- Chỉ dùng tài khoản phụ, chủ tài khoản tự đồng ý. API không chính thức: có thể bị checkpoint hoặc vỡ khi Facebook đổi.
