package settingsmessaging

import (
	"net/url"
	"regexp"
	"strings"
)

// 企业微信群机器人只允许官方 Webhook 地址；发送时由 key 重新拼接固定地址，避免被用作任意出站请求。
const (
	WeComWebhookHost = "qyapi.weixin.qq.com"
	WeComWebhookPath = "/cgi-bin/webhook/send"

	weComWebhookMaskPrefix = "Webhook ****"
	weComWebhookMaskTail   = 6
)

var weComWebhookKeyPattern = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

// WeComNotificationChannelSetting 企业微信群机器人通知配置。WebhookKeys 为各群机器人 Webhook 的 key（敏感信息）。
type WeComNotificationChannelSetting struct {
	Enabled     bool     `json:"enabled"`
	WebhookKeys []string `json:"webhook_keys"`
}

// WeComNotificationChannelPatch 企业微信通知配置补丁。WebhookKeys 为空时保持原有配置不变。
type WeComNotificationChannelPatch struct {
	Enabled     *bool     `json:"enabled"`
	WebhookKeys *[]string `json:"webhook_keys"`
}

// NormalizeWeComWebhookKey 接受完整 Webhook 地址或单独的 key，返回 key。
func NormalizeWeComWebhookKey(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil || !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Host, WeComWebhookHost) ||
			parsed.User != nil || parsed.Path != WeComWebhookPath || parsed.Fragment != "" {
			return "", false
		}
		value = strings.TrimSpace(parsed.Query().Get("key"))
	}
	if !weComWebhookKeyPattern.MatchString(value) {
		return "", false
	}
	return value, true
}

// MaskWeComWebhookKey 返回可展示、可写入日志的脱敏标识。
func MaskWeComWebhookKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= weComWebhookMaskTail {
		return weComWebhookMaskPrefix
	}
	return weComWebhookMaskPrefix + key[len(key)-weComWebhookMaskTail:]
}

// ResolveWeComWebhookTarget 把测试发送目标解析为 key：可以是新的 Webhook 地址 / key，或已配置项的脱敏标识。
func ResolveWeComWebhookTarget(target string, configured []string) (string, bool) {
	if key, ok := NormalizeWeComWebhookKey(target); ok {
		return key, true
	}
	target = strings.TrimSpace(target)
	for _, key := range configured {
		if MaskWeComWebhookKey(key) == target {
			return key, true
		}
	}
	return "", false
}

func normalizeWeComWebhookKeys(items []string) []string {
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key, ok := NormalizeWeComWebhookKey(item)
		if !ok {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

// parseWeComWebhookKeysInput 校验管理端提交的 Webhook 列表，任一非空项不合法即失败。
func parseWeComWebhookKeysInput(items []string) ([]string, bool) {
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		if _, ok := NormalizeWeComWebhookKey(item); !ok {
			return nil, false
		}
	}
	return normalizeWeComWebhookKeys(items), true
}

func weComWebhookLabels(keys []string) []string {
	labels := make([]string, 0, len(keys))
	for _, key := range keys {
		labels = append(labels, MaskWeComWebhookKey(key))
	}
	return labels
}
