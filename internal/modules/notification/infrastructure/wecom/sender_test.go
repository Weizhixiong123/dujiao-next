package wecom

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dujiao-next/internal/modules/notification/contract"
)

func TestSenderPostsTextMessageToWebhookKey(t *testing.T) {
	var gotKey, gotContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("key")
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		_ = json.Unmarshal(raw, &body)
		if body.MsgType != "text" {
			t.Errorf("unexpected msgtype %q", body.MsgType)
		}
		gotContent = body.Text.Content
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	sender := &Sender{client: server.Client(), endpoint: server.URL}
	key := "693a91f6-7xxx-4bc4-97a0-0ec2sifa5aaa"
	if err := sender.SendMessage(context.Background(), "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key="+key, "订单支付成功"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotKey != key || gotContent != "订单支付成功" {
		t.Fatalf("unexpected request key=%q content=%q", gotKey, gotContent)
	}
}

func TestSenderReportsWeComErrorsAndRejectsInvalidKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":93000,"errmsg":"invalid webhook url"}`))
	}))
	defer server.Close()
	sender := &Sender{client: server.Client(), endpoint: server.URL}

	err := sender.SendMessage(context.Background(), "abcdef12-3456", "hello")
	if !errors.Is(err, contract.ErrSendFailed) || !strings.Contains(err.Error(), "93000") {
		t.Fatalf("expected wecom error, got %v", err)
	}
	for _, bad := range []string{"", "short", "https://evil.example.com/cgi-bin/webhook/send?key=abcdef123456", "http://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=abcdef123456"} {
		if err := sender.SendMessage(context.Background(), bad, "hello"); !errors.Is(err, contract.ErrConfigInvalid) {
			t.Fatalf("expected invalid config for %q, got %v", bad, err)
		}
	}
}

func TestTruncateUTF8KeepsValidRunes(t *testing.T) {
	value := strings.Repeat("通知", 1000)
	got := truncateUTF8(value, maxContentBytes)
	if len(got) > maxContentBytes || !utf8.ValidString(got) {
		t.Fatalf("truncated content invalid: len=%d", len(got))
	}
}
