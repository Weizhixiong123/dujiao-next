package userauthhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	captcha "github.com/dujiao-next/internal/modules/captcha/contract"
	captchahttp "github.com/dujiao-next/internal/modules/captcha/transport/http"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/gin-gonic/gin"
)

type sourceTestTOTPService struct{}

func (sourceTestTOTPService) GetStatus(uint) (*UserTOTPStatus, error) {
	return &UserTOTPStatus{}, nil
}
func (sourceTestTOTPService) Setup(uint) (*UserTOTPSetupResult, error) {
	return &UserTOTPSetupResult{}, nil
}
func (sourceTestTOTPService) Enable(uint, string) (*UserTOTPEnableResult, error) {
	return &UserTOTPEnableResult{}, nil
}
func (sourceTestTOTPService) Disable(uint, string, bool) error { return nil }
func (sourceTestTOTPService) RegenerateRecoveryCodes(uint, string) ([]string, error) {
	return nil, nil
}
func (sourceTestTOTPService) VerifyChallengeCode(uint, string) error         { return nil }
func (sourceTestTOTPService) VerifyChallengeRecoveryCode(uint, string) error { return nil }

type sourceTest2FAAuth struct {
	source string
}

func (s sourceTest2FAAuth) ParseUserChallengeToken(string) (*UserChallengeClaims, error) {
	return &UserChallengeClaims{
		UserID:      42,
		JTI:         "source-test-jti",
		LoginSource: s.source,
	}, nil
}

func (sourceTest2FAAuth) CompleteLoginAfter2FA(context.Context, uint, bool) (*AuthLoginResult, error) {
	now := time.Now()
	return &AuthLoginResult{
		User: &userdomain.User{
			ID:    42,
			Email: "buyer@example.com",
		},
		Token:     "access-token",
		ExpiresAt: now.Add(time.Hour),
	}, nil
}

func (sourceTest2FAAuth) GetUserEmail(uint) (string, error) {
	return "buyer@example.com", nil
}

type sourceTestChallengeStore struct{}

func (sourceTestChallengeStore) IsRevoked(context.Context, string) bool { return false }
func (sourceTestChallengeStore) BumpFails(context.Context, string) int64 {
	return 0
}
func (sourceTestChallengeStore) Revoke(context.Context, string) {}

type sourceTestLoginRecord struct {
	source string
	status string
}

type sourceTestLoginRecorder struct {
	records []sourceTestLoginRecord
}

func (r *sourceTestLoginRecorder) Record(_ string, _ uint, status, _, source, _, _, _ string) {
	r.records = append(r.records, sourceTestLoginRecord{source: source, status: status})
}

func TestVerifyUser2FAPreservesExternalLoginSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, source := range []string{constants.LoginLogSourceGoogle, constants.LoginLogSourceTelegram} {
		t.Run(source, func(t *testing.T) {
			loginRecorder := &sourceTestLoginRecorder{}
			handler := NewUser2FAHandler(
				sourceTestTOTPService{},
				sourceTest2FAAuth{source: source},
				sourceTestChallengeStore{},
				loginRecorder,
			)
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(
				http.MethodPost,
				"/api/v1/auth/login/verify-2fa",
				bytes.NewBufferString(`{"challenge_token":"challenge","code":"123456"}`),
			)
			context.Request.Header.Set("Content-Type", "application/json")

			handler.VerifyUser2FA(context)

			if len(loginRecorder.records) != 1 {
				t.Fatalf("login records = %d, want 1", len(loginRecorder.records))
			}
			if loginRecorder.records[0].source != source {
				t.Fatalf("login source = %q, want %q", loginRecorder.records[0].source, source)
			}
			if loginRecorder.records[0].status != constants.LoginLogStatusSuccess {
				t.Fatalf("login status = %q", loginRecorder.records[0].status)
			}
		})
	}
}

type purchaseTestAuth struct {
	called bool
	result *AuthLoginResult
	err    error
}

func (*purchaseTestAuth) GetRegistrationEnabled(bool) (bool, error)      { return true, nil }
func (*purchaseTestAuth) GetEmailVerificationEnabled(bool) (bool, error) { return false, nil }
func (*purchaseTestAuth) Register(context.Context, string, string, string, string, bool, bool) (*userdomain.User, string, time.Time, error) {
	panic("unexpected registration")
}
func (*purchaseTestAuth) LoginStep1(context.Context, string, string, bool) (*AuthLoginResult, error) {
	panic("unexpected ordinary login")
}
func (a *purchaseTestAuth) LoginOrRegister(_ context.Context, req UserLoginRequest) (*AuthLoginResult, error) {
	a.called = true
	return a.result, a.err
}

type purchaseTestCaptcha struct{ err error }

func (v purchaseTestCaptcha) Verify(scene string, _ captchahttp.CaptchaPayloadRequest, _ string) error {
	if scene != constants.CaptchaSceneLogin {
		panic("wrong captcha scene")
	}
	return v.err
}

func TestPurchaseAuthCaptchaAndPendingRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, blocked := range []bool{true, false} {
		t.Run(map[bool]string{true: "captcha required", false: "registration pending"}[blocked], func(t *testing.T) {
			auth := &purchaseTestAuth{result: &AuthLoginResult{RegistrationRequired: true, EmailVerificationRequired: true}}
			verifier := purchaseTestCaptcha{}
			if blocked {
				verifier.err = captcha.ErrRequired
			}
			handler := NewUserLoginHandler(auth, auth, verifier, nil)
			router := gin.New()
			limits := 0
			limit := func(c *gin.Context) { limits++; c.Next() }
			RegisterUserPurchaseAuthRoutes(router.Group("/auth"), handler, limit, limit)
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/auth/login-or-register", bytes.NewBufferString(`{"email":"buyer@example.com","password":"Secret123","agreement_accepted":true}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, req)
			if limits != 2 {
				t.Fatalf("both rate limits must run, got %d", limits)
			}
			var body struct {
				StatusCode int                    `json:"status_code"`
				Data       map[string]interface{} `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if blocked {
				if auth.called || body.StatusCode != 400 {
					t.Fatalf("captcha bypass: called=%v response=%s", auth.called, recorder.Body.String())
				}
			} else {
				if !auth.called || body.Data["registration_required"] != true || body.Data["email_verification_required"] != true {
					t.Fatalf("bad pending response: %s", recorder.Body.String())
				}
				if _, ok := body.Data["token"]; ok {
					t.Fatal("pending registration must not expose an access token")
				}
			}
		})
	}
}
