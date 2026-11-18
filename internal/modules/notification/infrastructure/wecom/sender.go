package wecom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dujiao-next/internal/modules/notification/contract"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
)

const requestTimeout = 10 * time.Second

// 企业微信群机器人 text 消息内容上限 2048 字节。
const maxContentBytes = 2048

const maxResponseSize = 64 << 10

// Sender 通过企业微信群机器人 Webhook 发送纯文本通知。
type Sender struct {
	client   *http.Client
	endpoint string
}

var _ contract.WeComSender = (*Sender)(nil)

// New 创建企业微信群机器人通知发送器。
func New() *Sender {
	return &Sender{
		client: &http.Client{Timeout: requestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
		endpoint: "https://" + settingsmessaging.WeComWebhookHost + settingsmessaging.WeComWebhookPath,
	}
}

// SendMessage 向 webhookKey 对应的群机器人发送文本消息。
func (s *Sender) SendMessage(ctx context.Context, webhookKey, message string) error {
	key, ok := settingsmessaging.NormalizeWeComWebhookKey(webhookKey)
	message = strings.TrimSpace(message)
	if s == nil || !ok || message == "" {
		return contract.ErrConfigInvalid
	}
	body, err := json.Marshal(map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": truncateUTF8(message, maxContentBytes)},
	})
	if err != nil {
		return fmt.Errorf("%w: encode wecom message: %v", contract.ErrSendFailed, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint+"?key="+url.QueryEscape(key), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: build wecom request", contract.ErrSendFailed)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		// 不包装原始错误：其中的 URL 含 Webhook key。
		return fmt.Errorf("%w: wecom request failed", contract.ErrSendFailed)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("%w: read wecom response", contract.ErrSendFailed)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: wecom http status %d", contract.ErrSendFailed, resp.StatusCode)
	}
	var result struct {
		ErrCode *int   `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.ErrCode == nil {
		return fmt.Errorf("%w: invalid wecom response", contract.ErrSendFailed)
	}
	if *result.ErrCode != 0 {
		return fmt.Errorf("%w: wecom errcode=%d errmsg=%s", contract.ErrSendFailed, *result.ErrCode, strings.TrimSpace(result.ErrMsg))
	}
	return nil
}

func truncateUTF8(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}
