// [2026-11-24 Johnny Chu - Chu Hoàng Anh] PHASE-017 Q017-2=B — fbchat-login: đăng nhập Facebook cá nhân
// bằng email + mật khẩu (luồng Messenger Lite của mautrix-meta), duyệt trên điện thoại / mã 2FA.
//
// Giao tiếp với tiến trình cha (Cell zalo-hub) qua stdin/stdout, mỗi dòng một JSON — cùng khuôn với
// fbchat-bridge-e2ee:
//
//	→ {"id":1,"method":"hello"}
//	← {"id":1,"ok":true,"data":{"protocolVersion":1,"bridgeVersion":"…","capabilities":[…]}}
//	→ {"id":2,"method":"loginStart","params":{"platform":"android"}}
//	← {"id":2,"ok":true,"data":{"done":false,"step":{…LoginStep của mautrix…}}}
//	→ {"id":3,"method":"loginSubmit","params":{"input":{"<field id>":"<giá trị>"}}}
//	→ {"id":4,"method":"loginWait"}       // bước "duyệt trên thiết bị khác"
//	→ {"id":5,"method":"loginCancelStep"} // về bước trước
//	← {"id":n,"ok":true,"data":{"done":true,"uid":"…","cookies":{…},"nativeSession":{…}}}
//
// Luật (D017-3): mật khẩu / mã chỉ nằm trong bộ nhớ của tiến trình này; không ghi đĩa, không ghi log.
// Cookie chỉ trả về tiến trình cha qua stdout; Cell tự mã hoá. Log ra stderr, mức "warn" trở lên.
// Mỗi tiến trình một lượt đăng nhập; xong thì tiến trình cha đóng stdin.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/mautrix-meta/pkg/messagix"
	"go.mau.fi/mautrix-meta/pkg/messagix/bloks"
	"go.mau.fi/mautrix-meta/pkg/messagix/cookies"
	"go.mau.fi/mautrix-meta/pkg/messagix/types"
	"go.mau.fi/util/exhttp"
	"maunium.net/go/mautrix/bridgev2"
)

const protocolVersion = 1

var bridgeVersion = "0.1.0"

var capabilities = []string{"loginStart", "loginSubmit", "loginWait", "loginCancelStep"}

// Mỗi bước gọi mạng của Facebook; quá thì báo lỗi để Cell hỏi lại người dùng.
const stepTimeout = 3 * time.Minute

type request struct {
	ID     uint64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type response struct {
	ID        uint64      `json:"id"`
	OK        bool        `json:"ok"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	ErrorCode string      `json:"errorCode,omitempty"`
}

var (
	outMu  sync.Mutex
	client *messagix.Client
	logger zerolog.Logger
)

func write(v interface{}) {
	outMu.Lock()
	defer outMu.Unlock()
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func fail(id uint64, err error) {
	code := "LOGIN_FAILED"
	if errors.As(err, &bloks.CheckpointError{}) {
		// Lỗi cuối: tài khoản bị Facebook giữ lại kiểm tra. Không thử lại cùng thông tin.
		code = "CHECKPOINT"
	} else if errors.Is(err, context.DeadlineExceeded) {
		code = "TIMEOUT"
	}
	write(response{ID: id, OK: false, Error: err.Error(), ErrorCode: code})
}

func platformOf(name string) types.Platform {
	if name == "ios" {
		return types.MessengerLiteIOS
	}
	return types.MessengerLiteAndroid
}

// proceed chạy tiếp luồng bloks; trả về bước cần người dùng, hoặc cookie khi xong.
func proceed(id uint64, input map[string]string) {
	if client == nil {
		fail(id, fmt.Errorf("loginStart chưa chạy"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), stepTimeout)
	defer cancel()
	step, newCookies, err := client.MessengerLite.DoLoginSteps(ctx, input)
	if err != nil {
		fail(id, err)
		return
	}
	if step != nil {
		write(response{ID: id, OK: true, Data: map[string]interface{}{"done": false, "step": publicStep(step)}})
		return
	}
	all := map[string]string{}
	for k, v := range newCookies.GetAll() {
		all[string(k)] = v
	}
	data := map[string]interface{}{
		"done":    true,
		"uid":     strconv.FormatInt(newCookies.GetUserID(), 10),
		"cookies": all,
	}
	if ns := client.MessengerLite.GetNativeSession(); ns != nil {
		data["nativeSession"] = ns
	}
	write(response{ID: id, OK: true, Data: data})
}

// publicStep chỉ giữ phần Cell cần để vẽ form; bỏ CompleteParams (có con trỏ nội bộ).
func publicStep(s *bridgev2.LoginStep) map[string]interface{} {
	out := map[string]interface{}{"type": s.Type, "stepId": s.StepID, "instructions": s.Instructions}
	if s.UserInputParams != nil {
		out["fields"] = s.UserInputParams.Fields
	}
	if s.DisplayAndWaitParams != nil {
		out["display"] = s.DisplayAndWaitParams
	}
	return out
}

func handle(req *request) {
	switch req.Method {
	case "hello":
		write(response{ID: req.ID, OK: true, Data: map[string]interface{}{
			"protocolVersion": protocolVersion, "bridgeVersion": bridgeVersion, "capabilities": capabilities,
		}})
	case "loginStart":
		if client != nil {
			fail(req.ID, fmt.Errorf("mỗi tiến trình chỉ một lượt đăng nhập"))
			return
		}
		var p struct {
			Platform string `json:"platform"`
		}
		_ = json.Unmarshal(req.Params, &p)
		client = messagix.NewClient(&cookies.Cookies{Platform: platformOf(p.Platform)}, logger, &messagix.Config{
			ClientSettings: exhttp.ClientSettings{},
		})
		proceed(req.ID, nil)
	case "loginSubmit":
		var p struct {
			Input map[string]string `json:"input"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || len(p.Input) == 0 {
			fail(req.ID, fmt.Errorf("thiếu input"))
			return
		}
		proceed(req.ID, p.Input)
		// Xoá tham chiếu tới mật khẩu ngay khi dùng xong.
		for k := range p.Input {
			delete(p.Input, k)
		}
	case "loginWait":
		proceed(req.ID, nil)
	case "loginCancelStep":
		if client == nil {
			fail(req.ID, fmt.Errorf("loginStart chưa chạy"))
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), stepTimeout)
		err := client.MessengerLite.CancelLoginStep(ctx)
		cancel()
		if err != nil {
			fail(req.ID, err)
			return
		}
		proceed(req.ID, nil)
	default:
		fail(req.ID, fmt.Errorf("method lạ: %s", req.Method))
	}
}

func main() {
	level := zerolog.WarnLevel
	if os.Getenv("FB_LOGIN_LOG") == "debug" {
		// Chỉ dùng khi gỡ lỗi trên máy thử; log debug của bloks có thể chứa trường form.
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)
	logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var req request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			write(response{OK: false, Error: "dòng không phải JSON"})
			continue
		}
		// Tuần tự: luồng bloks giữ trạng thái, không chạy hai bước cùng lúc.
		handle(&req)
	}
}
