package settingsmessaging

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

const testWeComKey = "693a91f6-7a2b-4bc4-97a0-0ec2b1fa5aaa"

func TestNormalizeWeComWebhookKeyAcceptsOfficialURLOrKeyOnly(t *testing.T) {
	for _, raw := range []string{
		testWeComKey,
		"  https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + testWeComKey + "  ",
	} {
		if key, ok := NormalizeWeComWebhookKey(raw); !ok || key != testWeComKey {
			t.Fatalf("expected key from %q, got %q %v", raw, key, ok)
		}
	}
	for _, raw := range []string{
		"",
		"short",
		"http://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + testWeComKey,
		"https://evil.example.com/cgi-bin/webhook/send?key=" + testWeComKey,
		"https://qyapi.weixin.qq.com/other?key=" + testWeComKey,
		"https://user@qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + testWeComKey,
		"key with spaces 12345",
	} {
		if _, ok := NormalizeWeComWebhookKey(raw); ok {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestNotificationCenterWeComPatchValidatesAndKeepsSavedWebhooks(t *testing.T) {
	enabled := true
	webhooks := []string{"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + testWeComKey, testWeComKey, " "}
	next, err := ApplyNotificationCenterSettingPatch(NotificationCenterDefaultSetting(), NotificationCenterSettingPatch{
		Channels: &NotificationChannelsPatch{WeCom: &WeComNotificationChannelPatch{Enabled: &enabled, WebhookKeys: &webhooks}},
	})
	if err != nil {
		t.Fatalf("apply wecom patch: %v", err)
	}
	if !next.Channels.WeCom.Enabled || !reflect.DeepEqual(next.Channels.WeCom.WebhookKeys, []string{testWeComKey}) {
		t.Fatalf("unexpected wecom setting: %#v", next.Channels.WeCom)
	}

	empty := []string{}
	kept, err := ApplyNotificationCenterSettingPatch(next, NotificationCenterSettingPatch{
		Channels: &NotificationChannelsPatch{WeCom: &WeComNotificationChannelPatch{WebhookKeys: &empty}},
	})
	if err != nil || !reflect.DeepEqual(kept.Channels.WeCom.WebhookKeys, []string{testWeComKey}) {
		t.Fatalf("empty patch must keep saved webhooks: %#v %v", kept.Channels.WeCom, err)
	}

	bad := []string{"https://evil.example.com/hook?key=" + testWeComKey}
	if _, err := ApplyNotificationCenterSettingPatch(next, NotificationCenterSettingPatch{
		Channels: &NotificationChannelsPatch{WeCom: &WeComNotificationChannelPatch{WebhookKeys: &bad}},
	}); !errors.Is(err, ErrNotificationConfigInvalid) {
		t.Fatalf("expected invalid webhook error, got %v", err)
	}

	if _, err := ApplyNotificationCenterSettingPatch(NotificationCenterDefaultSetting(), NotificationCenterSettingPatch{
		Channels: &NotificationChannelsPatch{WeCom: &WeComNotificationChannelPatch{Enabled: &enabled}},
	}); !errors.Is(err, ErrNotificationConfigInvalid) {
		t.Fatalf("enabled wecom without webhook must fail, got %v", err)
	}
}

func TestNotificationCenterWeComMaskAndPersistenceRoundTrip(t *testing.T) {
	setting := NotificationCenterDefaultSetting()
	setting.Channels.WeCom = WeComNotificationChannelSetting{Enabled: true, WebhookKeys: []string{testWeComKey}}

	decoded := DecodeNotificationCenterSetting(jsonmap.JSON(NotificationCenterSettingToMap(setting)), NotificationCenterDefaultSetting())
	if !decoded.Channels.WeCom.Enabled || !reflect.DeepEqual(decoded.Channels.WeCom.WebhookKeys, []string{testWeComKey}) {
		t.Fatalf("round trip lost wecom setting: %#v", decoded.Channels.WeCom)
	}

	masked := MaskNotificationCenterSettingForAdmin(setting)
	wecom := settingsvalue.ToStringAnyMap(settingsvalue.ToStringAnyMap(masked["channels"])["wecom"])
	if keys, _ := wecom["webhook_keys"].([]string); len(keys) != 0 {
		t.Fatalf("admin response must not expose webhook keys: %#v", wecom)
	}
	labels, _ := wecom["webhook_labels"].([]string)
	if len(labels) != 1 || strings.Contains(labels[0], testWeComKey) || wecom["webhook_count"] != 1 {
		t.Fatalf("unexpected masked wecom: %#v", wecom)
	}
	if key, ok := ResolveWeComWebhookTarget(labels[0], setting.Channels.WeCom.WebhookKeys); !ok || key != testWeComKey {
		t.Fatalf("masked label must resolve to configured key, got %q %v", key, ok)
	}
}
