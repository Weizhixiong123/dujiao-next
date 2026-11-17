package captchahttp

import (
	captchaapp "github.com/dujiao-next/internal/modules/captcha/application"
)

// Verifier adapts the captcha module to HTTP transport contracts.
type Verifier struct {
	service *captchaapp.Service
}

func NewVerifier(service *captchaapp.Service) Verifier {
	return Verifier{service: service}
}

// SceneEnabled 报告指定场景当前是否真正要求人机验证。
func (v Verifier) SceneEnabled(scene string) (bool, error) {
	if v.service == nil {
		return false, nil
	}
	return v.service.IsSceneEnabled(scene)
}

func (v Verifier) Verify(scene string, payload CaptchaPayloadRequest, clientIP string) error {
	if v.service == nil {
		return nil
	}
	return v.service.Verify(scene, payload.ToCaptchaPayload(), clientIP)
}
