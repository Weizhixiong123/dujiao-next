package integrationtest

import (
	"errors"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/pquerna/otp/totp"
)

func TestPurchaseLoginRegistersOnceAndRequiresExistingPassword(t *testing.T) {
	svc, settings, db := setupTelegramOAuthTestService(t)
	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldRegistrationEnabled:      true,
		constants.SettingFieldEmailVerificationEnabled: false,
	}); err != nil {
		t.Fatal(err)
	}
	input := userauthapp.PurchaseLoginInput{Email: " New@Example.com ", Password: "Secret123", AgreementAccepted: true}
	res, err := svc.LoginOrRegister(input)
	if err != nil || res == nil || res.Token == "" || res.User.Email != "new@example.com" {
		t.Fatalf("new account: %+v, %v", res, err)
	}
	userID := res.User.ID
	res, err = svc.LoginOrRegister(input)
	if err != nil || res == nil || res.Token == "" || res.User.ID != userID {
		t.Fatalf("existing account: %+v, %v", res, err)
	}
	input.Password = "Wrong123"
	if _, err := svc.LoginOrRegister(input); !errors.Is(err, userauthapp.ErrInvalidCredentials) {
		t.Fatalf("wrong password must fail: %v", err)
	}
	if n := countUsers(t, db); n != 1 {
		t.Fatalf("want exactly one account, got %d", n)
	}
}

func TestPurchaseLoginPreservesRegistrationPolicies(t *testing.T) {
	svc, settings, db := setupTelegramOAuthTestService(t)
	input := userauthapp.PurchaseLoginInput{Email: "new@example.com", Password: "Secret123"}
	// Missing agreement is still pending and must never create a user or issue a token.
	res, err := svc.LoginOrRegister(input)
	if err != nil || res == nil || !res.RegistrationRequired || res.EmailVerificationRequired || res.Token != "" {
		t.Fatalf("agreement pending: %+v, %v", res, err)
	}
	if n := countUsers(t, db); n != 0 {
		t.Fatalf("pending registration created %d accounts", n)
	}
	// Email verification is on by default, but purchase registration skips the email code:
	// the purchase captcha is the only verification step.
	input.AgreementAccepted = true
	res, err = svc.LoginOrRegister(input)
	if err != nil || res == nil || res.Token == "" || res.RegistrationRequired {
		t.Fatalf("purchase registration without email code: %+v, %v", res, err)
	}
	if n := countUsers(t, db); n != 1 {
		t.Fatalf("want exactly one account, got %d", n)
	}
	input.Email = "closed@example.com"
	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldRegistrationEnabled:      false,
		constants.SettingFieldEmailVerificationEnabled: false,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LoginOrRegister(input); !errors.Is(err, userauthapp.ErrRegistrationDisabled) {
		t.Fatalf("closed registration: %v", err)
	}
	createActiveUser(t, userstore.New(db), "existing@example.com", "Secret123")
	input.Email = "existing@example.com"
	res, err = svc.LoginOrRegister(input)
	if err != nil || res == nil || res.Token == "" {
		t.Fatalf("existing login when registration closed: %+v, %v", res, err)
	}
}

func TestPurchaseLoginRequiresAgreementAndInvite(t *testing.T) {
	svc, db := setupInviteOnlyService(t)
	input := userauthapp.PurchaseLoginInput{Email: "invite@example.com", Password: "Secret123"}
	res, err := svc.LoginOrRegister(input)
	if err != nil || res == nil || !res.RegistrationRequired || !res.InviteCodeRequired || res.Token != "" {
		t.Fatalf("invite required: %+v, %v", res, err)
	}
	createInviteCode(t, db, "PURCHASE", 1)
	input.InviteCode = "PURCHASE"
	res, err = svc.LoginOrRegister(input)
	if err != nil || res == nil || !res.RegistrationRequired || countUsers(t, db) != 0 {
		t.Fatalf("agreement required: %+v, %v", res, err)
	}
	input.AgreementAccepted = true
	res, err = svc.LoginOrRegister(input)
	if err != nil || res == nil || res.Token == "" || countUsers(t, db) != 1 {
		t.Fatalf("complete registration: %+v, %v", res, err)
	}
}

func TestPurchaseLoginPreservesTwoFactorAndAccountStatus(t *testing.T) {
	svc, totpSvc, repo, _ := newUser2FATestServices(t)
	user := createActiveUser(t, repo, "2fa@example.com", "Secret123")
	setup, err := totpSvc.Setup(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := totpSvc.Enable(user.ID, code); err != nil {
		t.Fatal(err)
	}
	input := userauthapp.PurchaseLoginInput{Email: user.Email, Password: "Secret123"}
	res, err := svc.LoginOrRegister(input)
	if err != nil || res == nil || !res.RequiresTOTP || res.ChallengeToken == "" || res.Token != "" {
		t.Fatalf("two-factor challenge: %+v, %v", res, err)
	}
	user.Status = "disabled"
	if err := repo.Update(user); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LoginOrRegister(input); !errors.Is(err, userauthapp.ErrUserDisabled) {
		t.Fatalf("disabled account: %v", err)
	}
	user.Status = constants.UserStatusActive
	user.EmailVerifiedAt = nil
	if err := repo.Update(user); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LoginOrRegister(input); !errors.Is(err, userauthapp.ErrEmailNotVerified) {
		t.Fatalf("unverified account: %v", err)
	}
}
