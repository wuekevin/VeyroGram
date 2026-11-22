package main

// Story 1.11: the GENERATED Go client (packages/clients/go) drives the real API end to end. If the contract, the generator or the server drift apart,
// this fails. The same OpenAPI files generate the TypeScript, Kotlin and Swift clients, so a server that satisfies this client satisfies the contract
// those clients were generated from (their own compile checks run in CI; only the Go client is exercised against a live server).

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	vg "veyrogram/clients/go"
)

func TestGeneratedGoClientAgainstTheRealAPI(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	var access string
	anon := &vg.Client{BaseURL: r.base}
	c := &vg.Client{BaseURL: r.base, Token: func(context.Context) (string, error) { return access, nil }}
	re := regexp.MustCompile(`验证码 (\d{6})`)

	signIn := func(phone, device string) *vg.TokenResponse {
		t.Helper()
		_, _ = r.d.Exec("UPDATE login_event SET created_at = created_at - INTERVAL 5 MINUTE")
		before := len(re.FindAllStringSubmatch(r.p.stderr.String(), -1))
		sc, err := anon.RequestSmsCode(ctx, vg.SmsCodeRequest{Phone: phone, DeviceId: device})
		if err != nil || sc.Status == "" || sc.ExpiresIn <= 0 {
			t.Fatalf("RequestSmsCode: %+v %v", sc, err)
		}
		var code string
		eventually(t, "sms code", 3*time.Second, func() bool {
			all := re.FindAllStringSubmatch(r.p.stderr.String(), -1)
			if len(all) > before {
				code = all[len(all)-1][1]
				return true
			}
			return false
		})
		tok, err := anon.Login(ctx, vg.LoginRequest{Phone: phone, Code: code, Device: vg.LoginRequestDevice{Id: device, Class: "app"}})
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		return tok
	}

	tokA := signIn("+8613800138701", "device-aaaaaaaaaaaaaaaa")
	if tokA.AccessToken == "" || tokA.RefreshToken == "" || tokA.TokenType == "" || tokA.UserId == "" || !tokA.IsNewUser {
		t.Fatalf("login response not decoded into the typed model: %+v", tokA)
	}
	access = tokA.AccessToken

	devs, err := c.ListDevices(ctx)
	if err != nil || len(devs.Items) != 1 {
		t.Fatalf("ListDevices: %+v %v", devs, err)
	}
	im, err := c.GetImToken(ctx)
	if err != nil || im.Uid != tokA.UserId || im.Token == "" {
		t.Fatalf("GetImToken: %+v %v", im, err)
	}

	// path parameters, 204 answers and typed lists
	tokB := signIn("+8613800138702", "device-bbbbbbbbbbbbbbbb")
	access = tokA.AccessToken
	if err := c.BlockUser(ctx, vg.BlockUserParams{Uid: tokB.UserId}); err != nil {
		t.Fatalf("BlockUser: %v", err)
	}
	bl, err := c.ListBlocks(ctx)
	if err != nil || len(bl.Items) != 1 || bl.Items[0].Uid != tokB.UserId {
		t.Fatalf("ListBlocks: %+v %v", bl, err)
	}
	if err := c.UnblockUser(ctx, vg.UnblockUserParams{Uid: tokB.UserId}); err != nil {
		t.Fatalf("UnblockUser: %v", err)
	}

	// an open object (the message payload envelope) keeps the fields the contract allows beyond `type`
	_, err = c.SendMessage(ctx, vg.SendRequest{ToUid: tokB.UserId, ClientMsgNo: ulid(), Payload: vg.SendRequestPayload{Type: 1, Extra: map[string]any{"content": "hello from the generated client"}}})
	if err != nil {
		t.Fatalf("SendMessage with an open payload: %v", err)
	}

	// the standard error envelope becomes one typed error
	_, err = c.SyncMessages(ctx, vg.SyncMessagesParams{Peer: "bad peer!", AfterSeq: ptr(int64(0))})
	var ae *vg.Error
	if !errors.As(err, &ae) || ae.Status != 400 || ae.Code == "" || ae.TraceID == "" {
		t.Fatalf("a 400 must surface as *vg.Error with code and trace id: %#v", err)
	}
	if _, err = (&vg.Client{BaseURL: r.base}).ListDevices(ctx); !errors.As(err, &ae) || ae.Code != "not_signed_in" {
		t.Fatalf("no token: %v", err)
	}

	// rotating refresh, then the old refresh token is dead; logout kills the access token at once
	rt, err := anon.RefreshToken(ctx, vg.RefreshRequest{RefreshToken: tokA.RefreshToken})
	if err != nil || rt.RefreshToken == tokA.RefreshToken {
		t.Fatalf("RefreshToken: %+v %v", rt, err)
	}
	if _, err := anon.RefreshToken(ctx, vg.RefreshRequest{RefreshToken: tokA.RefreshToken}); !errors.As(err, &ae) || ae.Status != 401 {
		t.Fatalf("a rotated refresh token must be refused: %v", err)
	}
	access = rt.AccessToken
	_ = c.Logout(ctx) // the replayed refresh token above already revoked the device; either answer is acceptable here
	if _, err := c.ListDevices(ctx); !errors.As(err, &ae) || ae.Status != 401 || ae.Code != "invalid_token" {
		t.Fatalf("a token of a revoked device must be refused with invalid_token: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }
