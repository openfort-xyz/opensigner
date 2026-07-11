package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	testUser     = "user-1"
	testProvider = "default"
)

type fakeSmsProvider struct {
	lastPhone string
	lastCode  string
}

func (f *fakeSmsProvider) SendCode(phoneNumber, code string) error {
	f.lastPhone, f.lastCode = phoneNumber, code
	return nil
}

// setupTest gives each test a fresh in-memory database, a fixed encryption
// key, a recording SMS provider, and a WebAuthn RP.
func setupTest(t *testing.T) *fakeSmsProvider {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := migrateModels(testDB); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	db = testDB

	shareEncryptionKey, err = hex.DecodeString(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatalf("failed to set encryption key: %v", err)
	}
	if err := initWebauthn(); err != nil {
		t.Fatalf("failed to init webauthn: %v", err)
	}
	fake := &fakeSmsProvider{}
	smsProvider = fake
	return fake
}

// testServer wraps the real route mux with a fake auth layer so tests
// exercise routing, MFA gating, and handlers without JWTs.
func testServer(username string) http.Handler {
	mux := newMux()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), fieldUserId, username)
		ctx = context.WithValue(ctx, fieldAuthProvider, testProvider)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}

type reqOpt func(*http.Request)

func fromDevice(userAgent, ip string) reqOpt {
	return func(r *http.Request) {
		r.Header.Set("User-Agent", userAgent)
		r.RemoteAddr = ip + ":1234"
	}
}

func doReq(t *testing.T, h http.Handler, method, path, body string, opts ...reqOpt) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	fromDevice("device-a", "10.0.0.1")(req)
	for _, opt := range opts {
		opt(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
	return v
}

// seedTotpMethod inserts an already-verified TOTP method, bypassing the
// enrollment flow (which would consume the current TOTP timestep).
func seedTotpMethod(t *testing.T, username string) (methodID, secret string) {
	t.Helper()
	key, err := totp.Generate(totp.GenerateOpts{Issuer: totpIssuer, AccountName: username})
	if err != nil {
		t.Fatalf("failed to generate totp key: %v", err)
	}
	encrypted, err := encryptShare(key.Secret())
	if err != nil {
		t.Fatalf("failed to encrypt secret: %v", err)
	}
	method := MfaMethod{
		ID:           uuid.NewString(),
		Username:     username,
		AuthProvider: testProvider,
		Type:         mfaTypeTotp,
		Secret:       encrypted,
		Verified:     true,
	}
	if err := db.Create(&method).Error; err != nil {
		t.Fatalf("failed to seed method: %v", err)
	}
	return method.ID, key.Secret()
}

func currentTotpCode(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}
	return code
}

// totpVerify runs the challenge + verify flow and returns the verify response.
func totpVerify(t *testing.T, h http.Handler, methodID, secret string, opts ...reqOpt) *httptest.ResponseRecorder {
	t.Helper()
	rec := doReq(t, h, http.MethodPost, "/v1/mfa/challenges",
		fmt.Sprintf(`{"methodId":%q}`, methodID), opts...)
	if rec.Code != http.StatusCreated {
		t.Fatalf("challenge creation failed: %d %s", rec.Code, rec.Body.String())
	}
	challenge := decodeBody[MfaChallengeResponse](t, rec)
	return doReq(t, h, http.MethodPost, "/v1/mfa/verify",
		fmt.Sprintf(`{"challengeId":%q,"code":%q}`, challenge.ChallengeID, currentTotpCode(t, secret)), opts...)
}

func TestGateOpenWithoutEnrollment(t *testing.T) {
	setupTest(t)
	h := testServer(testUser)

	rec := doReq(t, h, http.MethodPost, "/v1/devices/init", `{"chainId":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for unenrolled user, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGateClosesAfterEnrollmentAndOpensAfterVerify(t *testing.T) {
	setupTest(t)
	h := testServer(testUser)
	methodID, secret := seedTotpMethod(t, testUser)

	rec := doReq(t, h, http.MethodPost, "/v1/devices/init", `{"chainId":1}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for enrolled user without session, got %d", rec.Code)
	}
	required := decodeBody[MfaRequiredResponse](t, rec)
	if required.Error != "mfa_required" || len(required.Methods) != 1 || required.Methods[0].ID != methodID {
		t.Fatalf("unexpected mfa_required body: %+v", required)
	}

	if rec := totpVerify(t, h, methodID, secret); rec.Code != http.StatusOK {
		t.Fatalf("verify failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(t, h, http.MethodPost, "/v1/devices/init", `{"chainId":1}`); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after verify, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSessionScopedToDeviceFingerprint(t *testing.T) {
	setupTest(t)
	h := testServer(testUser)
	methodID, secret := seedTotpMethod(t, testUser)

	if rec := totpVerify(t, h, methodID, secret); rec.Code != http.StatusOK {
		t.Fatalf("verify failed: %d %s", rec.Code, rec.Body.String())
	}
	rec := doReq(t, h, http.MethodPost, "/v1/devices/init", `{"chainId":1}`, fromDevice("device-b", "10.0.0.2"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 from another device, got %d", rec.Code)
	}
}

func TestTotpEnrollmentFlow(t *testing.T) {
	setupTest(t)
	h := testServer(testUser)

	rec := doReq(t, h, http.MethodPost, "/v1/mfa/enroll", `{"type":"totp"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll failed: %d %s", rec.Code, rec.Body.String())
	}
	enrolled := decodeBody[MfaEnrollResponse](t, rec)
	if enrolled.Secret == "" || !strings.HasPrefix(enrolled.OtpauthURL, "otpauth://totp/") {
		t.Fatalf("expected totp provisioning data, got %+v", enrolled)
	}

	rec = doReq(t, h, http.MethodPost, "/v1/mfa/enroll/verify",
		fmt.Sprintf(`{"methodId":%q,"code":%q}`, enrolled.MethodID, currentTotpCode(t, enrolled.Secret)))
	if rec.Code != http.StatusOK {
		t.Fatalf("enroll verify failed: %d %s", rec.Code, rec.Body.String())
	}
	verify := decodeBody[MfaVerifyResponse](t, rec)
	if !verify.Verified || verify.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("expected active session, got %+v", verify)
	}

	// The enrolling device holds a session, so the gate stays open for it.
	if rec := doReq(t, h, http.MethodPost, "/v1/devices/init", `{"chainId":1}`); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for enrolling device, got %d", rec.Code)
	}
}

func TestTotpCodeReplayRejected(t *testing.T) {
	setupTest(t)
	h := testServer(testUser)
	methodID, secret := seedTotpMethod(t, testUser)

	if rec := totpVerify(t, h, methodID, secret); rec.Code != http.StatusOK {
		t.Fatalf("first verify failed: %d %s", rec.Code, rec.Body.String())
	}
	rec := totpVerify(t, h, methodID, secret, fromDevice("device-b", "10.0.0.2"))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "already used") {
		t.Fatalf("expected replay rejection, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSmsEnrollmentAndAttemptLimit(t *testing.T) {
	fake := setupTest(t)
	h := testServer(testUser)

	rec := doReq(t, h, http.MethodPost, "/v1/mfa/enroll", `{"type":"sms","phoneNumber":"+15551234567"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll failed: %d %s", rec.Code, rec.Body.String())
	}
	enrolled := decodeBody[MfaEnrollResponse](t, rec)
	if fake.lastPhone != "+15551234567" || len(fake.lastCode) != 6 {
		t.Fatalf("expected SMS delivery, got phone=%q code=%q", fake.lastPhone, fake.lastCode)
	}

	rec = doReq(t, h, http.MethodPost, "/v1/mfa/enroll/verify",
		fmt.Sprintf(`{"methodId":%q,"challengeId":%q,"code":%q}`, enrolled.MethodID, enrolled.ChallengeID, fake.lastCode))
	if rec.Code != http.StatusOK {
		t.Fatalf("enroll verify failed: %d %s", rec.Code, rec.Body.String())
	}

	// New verification challenge: burn all attempts with wrong codes, then
	// even the right code must be rejected.
	rec = doReq(t, h, http.MethodPost, "/v1/mfa/challenges", fmt.Sprintf(`{"methodId":%q}`, enrolled.MethodID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("challenge failed: %d %s", rec.Code, rec.Body.String())
	}
	challenge := decodeBody[MfaChallengeResponse](t, rec)
	for i := 0; i < mfaMaxAttempts; i++ {
		rec = doReq(t, h, http.MethodPost, "/v1/mfa/verify",
			fmt.Sprintf(`{"challengeId":%q,"code":"000000"}`, challenge.ChallengeID))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid code") {
			t.Fatalf("attempt %d: expected invalid code, got %d: %s", i+1, rec.Code, rec.Body.String())
		}
	}
	rec = doReq(t, h, http.MethodPost, "/v1/mfa/verify",
		fmt.Sprintf(`{"challengeId":%q,"code":%q}`, challenge.ChallengeID, fake.lastCode))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "too many attempts") {
		t.Fatalf("expected attempt limit, got %d: %s", rec.Code, rec.Body.String())
	}

	// A fresh challenge issues a fresh code that works.
	rec = doReq(t, h, http.MethodPost, "/v1/mfa/challenges", fmt.Sprintf(`{"methodId":%q}`, enrolled.MethodID))
	challenge = decodeBody[MfaChallengeResponse](t, rec)
	rec = doReq(t, h, http.MethodPost, "/v1/mfa/verify",
		fmt.Sprintf(`{"challengeId":%q,"code":%q}`, challenge.ChallengeID, fake.lastCode))
	if rec.Code != http.StatusOK {
		t.Fatalf("fresh challenge verify failed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestChallengeExpiry(t *testing.T) {
	fake := setupTest(t)
	h := testServer(testUser)
	methodID := seedSmsMethod(t, testUser)

	rec := doReq(t, h, http.MethodPost, "/v1/mfa/challenges", fmt.Sprintf(`{"methodId":%q}`, methodID))
	challenge := decodeBody[MfaChallengeResponse](t, rec)
	db.Model(&MfaChallenge{}).Where("id = ?", challenge.ChallengeID).
		Update("expires_at", time.Now().Add(-time.Minute))

	rec = doReq(t, h, http.MethodPost, "/v1/mfa/verify",
		fmt.Sprintf(`{"challengeId":%q,"code":%q}`, challenge.ChallengeID, fake.lastCode))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "expired") {
		t.Fatalf("expected expiry rejection, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChallengeCancellation(t *testing.T) {
	fake := setupTest(t)
	h := testServer(testUser)
	methodID := seedSmsMethod(t, testUser)

	rec := doReq(t, h, http.MethodPost, "/v1/mfa/challenges", fmt.Sprintf(`{"methodId":%q}`, methodID))
	challenge := decodeBody[MfaChallengeResponse](t, rec)

	rec = doReq(t, h, http.MethodPost, fmt.Sprintf("/v1/mfa/challenges/%s/cancel", challenge.ChallengeID), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel failed: %d %s", rec.Code, rec.Body.String())
	}
	cancelled := decodeBody[MfaChallengeResponse](t, rec)
	if cancelled.Status != mfaStatusCancelled {
		t.Fatalf("expected cancelled status, got %+v", cancelled)
	}

	rec = doReq(t, h, http.MethodPost, "/v1/mfa/verify",
		fmt.Sprintf(`{"challengeId":%q,"code":%q}`, challenge.ChallengeID, fake.lastCode))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "cancelled") {
		t.Fatalf("expected cancelled rejection, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnrollmentRevokesOtherSessions(t *testing.T) {
	setupTest(t)
	h := testServer(testUser)
	methodID, secret := seedTotpMethod(t, testUser)

	deviceB := fromDevice("device-b", "10.0.0.2")
	if rec := totpVerify(t, h, methodID, secret, deviceB); rec.Code != http.StatusOK {
		t.Fatalf("device B verify failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(t, h, http.MethodPost, "/v1/devices/init", `{"chainId":1}`, deviceB); rec.Code != http.StatusOK {
		t.Fatalf("expected device B gate open, got %d", rec.Code)
	}

	// Device B (holding a session) enrolls an SMS method; every other
	// session must be revoked when it activates.
	rec := doReq(t, h, http.MethodPost, "/v1/mfa/enroll", `{"type":"sms","phoneNumber":"+15551234567"}`, deviceB)
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll failed: %d %s", rec.Code, rec.Body.String())
	}
	enrolled := decodeBody[MfaEnrollResponse](t, rec)
	fake := smsProvider.(*fakeSmsProvider)
	rec = doReq(t, h, http.MethodPost, "/v1/mfa/enroll/verify",
		fmt.Sprintf(`{"methodId":%q,"challengeId":%q,"code":%q}`, enrolled.MethodID, enrolled.ChallengeID, fake.lastCode), deviceB)
	if rec.Code != http.StatusOK {
		t.Fatalf("enroll verify failed: %d %s", rec.Code, rec.Body.String())
	}

	// Device B just re-verified, so it keeps access; a session device A had
	// obtained before enrollment would now be revoked.
	if rec := doReq(t, h, http.MethodPost, "/v1/devices/init", `{"chainId":1}`, deviceB); rec.Code != http.StatusOK {
		t.Fatalf("expected device B to keep access, got %d", rec.Code)
	}
	var sessions int64
	db.Model(&MfaSession{}).Where("username = ?", testUser).Count(&sessions)
	if sessions != 1 {
		t.Fatalf("expected exactly 1 session after enrollment revocation, got %d", sessions)
	}
}

func TestUnenrollRequiresSessionAndDisablesMfa(t *testing.T) {
	setupTest(t)
	h := testServer(testUser)
	methodID, secret := seedTotpMethod(t, testUser)

	rec := doReq(t, h, http.MethodDelete, "/v1/mfa/methods/"+methodID, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected unenroll to require MFA, got %d", rec.Code)
	}

	if rec := totpVerify(t, h, methodID, secret); rec.Code != http.StatusOK {
		t.Fatalf("verify failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = doReq(t, h, http.MethodDelete, "/v1/mfa/methods/"+methodID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unenroll failed: %d %s", rec.Code, rec.Body.String())
	}

	// MFA is disabled again: a device with no session passes the gate.
	rec = doReq(t, h, http.MethodPost, "/v1/devices/init", `{"chainId":1}`, fromDevice("device-c", "10.0.0.3"))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected gate open after unenroll, got %d", rec.Code)
	}
}

func TestEnrollGatedOnceEnrolled(t *testing.T) {
	setupTest(t)
	h := testServer(testUser)
	seedTotpMethod(t, testUser)

	// A stolen JWT (no MFA session) cannot add methods once MFA is on.
	rec := doReq(t, h, http.MethodPost, "/v1/mfa/enroll", `{"type":"totp"}`, fromDevice("attacker", "6.6.6.6"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected enroll to be gated, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPasskeyEnrollReturnsWebauthnOptions(t *testing.T) {
	setupTest(t)
	h := testServer(testUser)

	rec := doReq(t, h, http.MethodPost, "/v1/mfa/enroll", `{"type":"passkey"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll failed: %d %s", rec.Code, rec.Body.String())
	}
	enrolled := decodeBody[MfaEnrollResponse](t, rec)
	var options struct {
		Challenge string `json:"challenge"`
		Rp        struct {
			ID string `json:"id"`
		} `json:"rp"`
	}
	if err := json.Unmarshal(enrolled.PublicKey, &options); err != nil {
		t.Fatalf("failed to decode creation options: %v", err)
	}
	if options.Challenge == "" || options.Rp.ID != "localhost" {
		t.Fatalf("unexpected creation options: %s", enrolled.PublicKey)
	}

	rec = doReq(t, h, http.MethodPost, "/v1/mfa/enroll/verify",
		fmt.Sprintf(`{"methodId":%q,"challengeId":%q,"credential":{"garbage":true}}`, enrolled.MethodID, enrolled.ChallengeID))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid credential rejection, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMultiSignerScope(t *testing.T) {
	setupTest(t)
	h := testServer(testUser)
	seedTotpMethod(t, testUser)

	// MFA is user-scoped: every share-retrieval endpoint is gated, no matter
	// which signer or account the request targets.
	for _, path := range []string{"/v2/devices/recover", "/v1/devices/init"} {
		rec := doReq(t, h, http.MethodPost, path, `{"chainId":1,"account":"acc-1"}`)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: expected 403, got %d", path, rec.Code)
		}
	}

	// Another user is unaffected.
	other := testServer("user-2")
	if rec := doReq(t, other, http.MethodPost, "/v1/devices/init", `{"chainId":1}`); rec.Code != http.StatusOK {
		t.Fatalf("expected other user unaffected, got %d", rec.Code)
	}
}

func seedSmsMethod(t *testing.T, username string) string {
	t.Helper()
	encrypted, err := encryptShare("+15551234567")
	if err != nil {
		t.Fatalf("failed to encrypt phone: %v", err)
	}
	method := MfaMethod{
		ID:           uuid.NewString(),
		Username:     username,
		AuthProvider: testProvider,
		Type:         mfaTypeSms,
		Secret:       encrypted,
		PhoneHint:    "***4567",
		Verified:     true,
	}
	if err := db.Create(&method).Error; err != nil {
		t.Fatalf("failed to seed method: %v", err)
	}
	return method.ID
}
