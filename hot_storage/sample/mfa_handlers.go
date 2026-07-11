package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

type apiError struct {
	status  int
	message string
}

func internalError(message string) *apiError {
	return &apiError{status: http.StatusInternalServerError, message: message}
}

func badRequest(message string) *apiError {
	return &apiError{status: http.StatusBadRequest, message: message}
}

func writeApiError(w http.ResponseWriter, apiErr *apiError) {
	http.Error(w, apiErr.message, apiErr.status)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set(contentTypeHeader, contentTypeJSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func userFromContext(r *http.Request) (string, string, bool) {
	username, uok := r.Context().Value(fieldUserId).(string)
	authProvider, pok := r.Context().Value(fieldAuthProvider).(string)
	return username, authProvider, uok && pok && username != ""
}

func handleMfaListMethods(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username, authProvider, ok := userFromContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	methods, err := verifiedMfaMethods(username, authProvider)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	infos := mfaMethodInfos(methods)
	writeJSON(w, http.StatusOK, MfaMethodListResponse{
		Object: "list",
		URL:    "/v1/mfa/methods",
		Data:   infos,
		Total:  len(infos),
	})
}

func handleMfaEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req MfaEnrollRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	username, authProvider, ok := userFromContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var resp *MfaEnrollResponse
	var apiErr *apiError
	switch req.Type {
	case mfaTypeTotp:
		resp, apiErr = enrollTotp(username, authProvider)
	case mfaTypeSms:
		resp, apiErr = enrollSms(username, authProvider, req.PhoneNumber)
	case mfaTypePasskey:
		resp, apiErr = enrollPasskey(username, authProvider)
	default:
		apiErr = badRequest("unsupported mfa type (supported: totp, sms, passkey)")
	}
	if apiErr != nil {
		writeApiError(w, apiErr)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func enrollTotp(username, authProvider string) (*MfaEnrollResponse, *apiError) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: totpIssuer, AccountName: username})
	if err != nil {
		return nil, internalError("failed to generate totp secret")
	}
	encrypted, err := encryptShare(key.Secret())
	if err != nil {
		return nil, internalError("failed to encrypt totp secret")
	}
	method := MfaMethod{
		ID:           uuid.NewString(),
		Username:     username,
		AuthProvider: authProvider,
		Type:         mfaTypeTotp,
		Secret:       encrypted,
	}
	if err := db.Create(&method).Error; err != nil {
		return nil, internalError("failed to create mfa method")
	}
	return &MfaEnrollResponse{
		MethodID:   method.ID,
		Type:       mfaTypeTotp,
		Secret:     key.Secret(),
		OtpauthURL: key.URL(),
	}, nil
}

func enrollSms(username, authProvider, phoneNumber string) (*MfaEnrollResponse, *apiError) {
	if !validPhoneNumber(phoneNumber) {
		return nil, badRequest("phoneNumber must be in E.164 format (e.g. +15551234567)")
	}
	encrypted, err := encryptShare(phoneNumber)
	if err != nil {
		return nil, internalError("failed to encrypt phone number")
	}
	method := MfaMethod{
		ID:           uuid.NewString(),
		Username:     username,
		AuthProvider: authProvider,
		Type:         mfaTypeSms,
		Secret:       encrypted,
		PhoneHint:    "***" + phoneNumber[len(phoneNumber)-4:],
	}
	if err := db.Create(&method).Error; err != nil {
		return nil, internalError("failed to create mfa method")
	}

	challenge := &MfaChallenge{
		Username:     username,
		AuthProvider: authProvider,
		MethodID:     method.ID,
		Type:         mfaTypeSms,
		Purpose:      mfaPurposeEnroll,
	}
	send, apiErr := attachSmsCode(challenge, &method)
	if apiErr != nil {
		return nil, apiErr
	}
	if err := persistMfaChallenge(challenge); err != nil {
		return nil, internalError("failed to create challenge")
	}
	if err := send(); err != nil {
		slog.Error("failed to send MFA SMS", slog.String("error", err.Error()))
		db.Delete(challenge)
		db.Delete(&method)
		return nil, internalError("failed to send verification code")
	}
	return &MfaEnrollResponse{
		MethodID:    method.ID,
		Type:        mfaTypeSms,
		ChallengeID: challenge.ID,
	}, nil
}

func enrollPasskey(username, authProvider string) (*MfaEnrollResponse, *apiError) {
	methods, err := verifiedMfaMethods(username, authProvider)
	if err != nil {
		return nil, internalError("database error")
	}
	user, err := loadWebauthnUser(username, authProvider, methods)
	if err != nil {
		return nil, internalError("failed to load webauthn user")
	}

	creation, sessionData, err := webauthnRP.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementPreferred,
			UserVerification: protocol.VerificationRequired,
		}),
		webauthn.WithExclusions(credentialDescriptors(user.credentials)),
	)
	if err != nil {
		return nil, internalError("failed to begin passkey registration")
	}
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return nil, internalError("failed to encode webauthn session")
	}
	optionsJSON, err := json.Marshal(creation.Response)
	if err != nil {
		return nil, internalError("failed to encode webauthn options")
	}

	method := MfaMethod{
		ID:           uuid.NewString(),
		Username:     username,
		AuthProvider: authProvider,
		Type:         mfaTypePasskey,
	}
	if err := db.Create(&method).Error; err != nil {
		return nil, internalError("failed to create mfa method")
	}
	challenge := &MfaChallenge{
		Username:     username,
		AuthProvider: authProvider,
		MethodID:     method.ID,
		Type:         mfaTypePasskey,
		Purpose:      mfaPurposeEnroll,
		WebauthnData: string(sessionJSON),
	}
	if err := persistMfaChallenge(challenge); err != nil {
		return nil, internalError("failed to create challenge")
	}
	return &MfaEnrollResponse{
		MethodID:    method.ID,
		Type:        mfaTypePasskey,
		ChallengeID: challenge.ID,
		PublicKey:   optionsJSON,
	}, nil
}

func handleMfaEnrollVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req MfaEnrollVerifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	username, authProvider, ok := userFromContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var method MfaMethod
	if err := db.First(&method, "id = ? AND username = ? AND auth_provider = ?",
		req.MethodID, username, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusBadRequest)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if method.Verified {
		http.Error(w, "method already verified", http.StatusConflict)
		return
	}

	var apiErr *apiError
	switch method.Type {
	case mfaTypeTotp:
		apiErr = verifyTotpCode(&method, req.Code)
	case mfaTypeSms:
		apiErr = confirmSmsEnrollment(&method, req)
	case mfaTypePasskey:
		apiErr = confirmPasskeyEnrollment(&method, req)
	default:
		apiErr = badRequest("unsupported mfa type")
	}
	if apiErr != nil {
		writeApiError(w, apiErr)
		return
	}

	method.Verified = true
	if err := db.Save(&method).Error; err != nil {
		http.Error(w, "failed to activate mfa method", http.StatusInternalServerError)
		return
	}
	// Enrollment revokes every cached MFA session for the user so all other
	// active sessions are challenged again (cross-session enforcement).
	session, err := activateMfaSession(r, username, authProvider, true)
	if err != nil {
		http.Error(w, "failed to create mfa session", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, MfaVerifyResponse{Verified: true, ExpiresAt: session.ExpiresAt.Unix()})
}

func confirmSmsEnrollment(method *MfaMethod, req MfaEnrollVerifyRequest) *apiError {
	challenge, apiErr := loadChallengeForAttempt(req.ChallengeID, method.Username, method.AuthProvider, mfaPurposeEnroll)
	if apiErr != nil {
		return apiErr
	}
	if challenge.MethodID != method.ID {
		return badRequest(errNotFound)
	}
	if apiErr := verifySmsCode(challenge, req.Code); apiErr != nil {
		return apiErr
	}
	return markChallengeVerified(challenge)
}

func confirmPasskeyEnrollment(method *MfaMethod, req MfaEnrollVerifyRequest) *apiError {
	challenge, apiErr := loadChallengeForAttempt(req.ChallengeID, method.Username, method.AuthProvider, mfaPurposeEnroll)
	if apiErr != nil {
		return apiErr
	}
	if challenge.MethodID != method.ID {
		return badRequest(errNotFound)
	}
	if len(req.Credential) == 0 {
		return badRequest("credential is required")
	}

	var sessionData webauthn.SessionData
	if err := json.Unmarshal([]byte(challenge.WebauthnData), &sessionData); err != nil {
		return internalError("failed to decode webauthn session")
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(req.Credential))
	if err != nil {
		return badRequest("invalid credential")
	}
	user, err := loadWebauthnUser(method.Username, method.AuthProvider, nil)
	if err != nil {
		return internalError("failed to load webauthn user")
	}
	credential, err := webauthnRP.CreateCredential(user, sessionData, parsed)
	if err != nil {
		return badRequest("invalid credential")
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return internalError("failed to encode credential")
	}
	method.Credential = string(credentialJSON)
	return markChallengeVerified(challenge)
}

func handleMfaCreateChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req MfaChallengeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	username, authProvider, ok := userFromContext(r)
	if !ok {
		unauthorized(w)
		return
	}
	if req.MethodID == "" {
		http.Error(w, "methodId is required", http.StatusBadRequest)
		return
	}

	var method MfaMethod
	if err := db.First(&method, "id = ? AND username = ? AND auth_provider = ? AND verified = ?",
		req.MethodID, username, authProvider, true).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusBadRequest)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	challenge := &MfaChallenge{
		Username:     username,
		AuthProvider: authProvider,
		MethodID:     method.ID,
		Type:         method.Type,
		Purpose:      mfaPurposeVerify,
	}
	var publicKey json.RawMessage
	var send func() error
	var apiErr *apiError
	switch method.Type {
	case mfaTypeSms:
		send, apiErr = attachSmsCode(challenge, &method)
	case mfaTypePasskey:
		publicKey, apiErr = attachPasskeyAssertion(challenge, username, authProvider)
	}
	if apiErr != nil {
		writeApiError(w, apiErr)
		return
	}
	if err := persistMfaChallenge(challenge); err != nil {
		http.Error(w, "failed to create challenge", http.StatusInternalServerError)
		return
	}
	if send != nil {
		if err := send(); err != nil {
			slog.Error("failed to send MFA SMS", slog.String("error", err.Error()))
			db.Delete(challenge)
			http.Error(w, "failed to send verification code", http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusCreated, MfaChallengeResponse{
		ChallengeID: challenge.ID,
		Type:        challenge.Type,
		Status:      challenge.Status,
		ExpiresAt:   challenge.ExpiresAt.Unix(),
		PublicKey:   publicKey,
	})
}

// attachSmsCode generates a code, stores its hash on the challenge, and
// returns a closure that delivers it (so the caller can persist first).
func attachSmsCode(challenge *MfaChallenge, method *MfaMethod) (func() error, *apiError) {
	code, err := generateSmsCode()
	if err != nil {
		return nil, internalError("failed to generate code")
	}
	phoneNumber, err := decryptShare(method.Secret)
	if err != nil {
		return nil, internalError("failed to decrypt phone number")
	}
	challenge.CodeHash = hashMfaCode(code)
	return func() error { return smsProvider.SendCode(phoneNumber, code) }, nil
}

func attachPasskeyAssertion(challenge *MfaChallenge, username, authProvider string) (json.RawMessage, *apiError) {
	methods, err := verifiedMfaMethods(username, authProvider)
	if err != nil {
		return nil, internalError("database error")
	}
	user, err := loadWebauthnUser(username, authProvider, methods)
	if err != nil {
		return nil, internalError("failed to load webauthn user")
	}
	if len(user.credentials) == 0 {
		return nil, badRequest("no passkey credentials enrolled")
	}
	assertion, sessionData, err := webauthnRP.BeginLogin(user,
		webauthn.WithAllowedCredentials(credentialDescriptors(user.credentials)))
	if err != nil {
		return nil, internalError("failed to begin passkey login")
	}
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return nil, internalError("failed to encode webauthn session")
	}
	optionsJSON, err := json.Marshal(assertion.Response)
	if err != nil {
		return nil, internalError("failed to encode webauthn options")
	}
	challenge.WebauthnData = string(sessionJSON)
	return optionsJSON, nil
}

func handleMfaVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req MfaVerifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	username, authProvider, ok := userFromContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	challenge, apiErr := loadChallengeForAttempt(req.ChallengeID, username, authProvider, mfaPurposeVerify)
	if apiErr != nil {
		writeApiError(w, apiErr)
		return
	}
	var method MfaMethod
	if err := db.First(&method, "id = ? AND username = ? AND auth_provider = ? AND verified = ?",
		challenge.MethodID, username, authProvider, true).Error; err != nil {
		http.Error(w, errNotFound, http.StatusBadRequest)
		return
	}

	switch method.Type {
	case mfaTypeTotp:
		apiErr = verifyTotpCode(&method, req.Code)
	case mfaTypeSms:
		apiErr = verifySmsCode(challenge, req.Code)
	case mfaTypePasskey:
		apiErr = verifyPasskeyAssertion(challenge, username, authProvider, req.Credential)
	default:
		apiErr = badRequest("unsupported mfa type")
	}
	if apiErr != nil {
		writeApiError(w, apiErr)
		return
	}
	if apiErr := markChallengeVerified(challenge); apiErr != nil {
		writeApiError(w, apiErr)
		return
	}
	session, err := activateMfaSession(r, username, authProvider, false)
	if err != nil {
		http.Error(w, "failed to create mfa session", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, MfaVerifyResponse{Verified: true, ExpiresAt: session.ExpiresAt.Unix()})
}

func handleMfaCancelChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	challengeId := r.PathValue("challengeId")
	username, authProvider, ok := userFromContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var challenge MfaChallenge
	if err := db.First(&challenge, "id = ? AND username = ? AND auth_provider = ?",
		challengeId, username, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusNotFound)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if challenge.Status != mfaStatusPending {
		http.Error(w, "challenge is not pending", http.StatusBadRequest)
		return
	}
	challenge.Status = mfaStatusCancelled
	if err := db.Save(&challenge).Error; err != nil {
		http.Error(w, "failed to cancel challenge", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, MfaChallengeResponse{
		ChallengeID: challenge.ID,
		Type:        challenge.Type,
		Status:      challenge.Status,
		ExpiresAt:   challenge.ExpiresAt.Unix(),
	})
}

func handleMfaDeleteMethod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	methodId := r.PathValue("methodId")
	username, authProvider, ok := userFromContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var method MfaMethod
	if err := db.First(&method, "id = ? AND username = ? AND auth_provider = ?",
		methodId, username, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusNotFound)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if err := db.Delete(&method).Error; err != nil {
		http.Error(w, "failed to delete mfa method", http.StatusInternalServerError)
		return
	}

	remaining, err := verifiedMfaMethods(username, authProvider)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if len(remaining) == 0 {
		if err := revokeAllMfaSessions(username, authProvider); err != nil {
			http.Error(w, "failed to revoke mfa sessions", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// persistMfaChallenge stamps identity, status, and expiry on a prepared
// challenge and stores it, opportunistically dropping the user's expired
// challenges.
func persistMfaChallenge(challenge *MfaChallenge) error {
	db.Where("username = ? AND auth_provider = ? AND expires_at <= ?",
		challenge.Username, challenge.AuthProvider, time.Now()).Delete(&MfaChallenge{})
	challenge.ID = uuid.NewString()
	challenge.Status = mfaStatusPending
	challenge.ExpiresAt = time.Now().Add(mfaChallengeTTL)
	return db.Create(challenge).Error
}

// loadChallengeForAttempt fetches a pending challenge and consumes one
// attempt. The attempt is counted (and persisted) before any code or
// credential comparison so failures cannot be retried for free.
func loadChallengeForAttempt(challengeID, username, authProvider, purpose string) (*MfaChallenge, *apiError) {
	if challengeID == "" {
		return nil, badRequest("challengeId is required")
	}
	var challenge MfaChallenge
	if err := db.First(&challenge, "id = ? AND username = ? AND auth_provider = ? AND purpose = ?",
		challengeID, username, authProvider, purpose).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, badRequest(errNotFound)
		}
		return nil, internalError("database error")
	}
	switch {
	case challenge.Status == mfaStatusCancelled:
		return nil, badRequest("challenge was cancelled")
	case challenge.Status != mfaStatusPending:
		return nil, badRequest("challenge is not pending")
	case time.Now().After(challenge.ExpiresAt):
		return nil, badRequest("challenge expired, request a new one")
	}
	challenge.Attempts++
	if err := db.Save(&challenge).Error; err != nil {
		return nil, internalError("database error")
	}
	if challenge.Attempts > mfaMaxAttempts {
		return nil, badRequest("too many attempts, request a new challenge")
	}
	return &challenge, nil
}

func markChallengeVerified(challenge *MfaChallenge) *apiError {
	challenge.Status = mfaStatusVerified
	if err := db.Save(challenge).Error; err != nil {
		return internalError("failed to update challenge")
	}
	return nil
}

// verifyTotpCode validates an RFC 6238 code and guards against replay: a
// code is only accepted once per 30-second timestep.
func verifyTotpCode(method *MfaMethod, code string) *apiError {
	if code == "" {
		return badRequest("code is required")
	}
	secret, err := decryptShare(method.Secret)
	if err != nil {
		return internalError("failed to decrypt totp secret")
	}
	step := time.Now().Unix() / totpPeriod
	if method.LastUsedStep >= step {
		return badRequest("code already used, wait for the next one")
	}
	if !totp.Validate(code, secret) {
		return badRequest("invalid code")
	}
	method.LastUsedStep = step
	if err := db.Save(method).Error; err != nil {
		return internalError("failed to update mfa method")
	}
	return nil
}

func verifySmsCode(challenge *MfaChallenge, code string) *apiError {
	if code == "" {
		return badRequest("code is required")
	}
	if !mfaCodeMatches(code, challenge.CodeHash) {
		return badRequest("invalid code")
	}
	return nil
}

func verifyPasskeyAssertion(challenge *MfaChallenge, username, authProvider string, credentialJSON json.RawMessage) *apiError {
	if len(credentialJSON) == 0 {
		return badRequest("credential is required")
	}
	var sessionData webauthn.SessionData
	if err := json.Unmarshal([]byte(challenge.WebauthnData), &sessionData); err != nil {
		return internalError("failed to decode webauthn session")
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(credentialJSON))
	if err != nil {
		return badRequest("invalid credential")
	}
	methods, err := verifiedMfaMethods(username, authProvider)
	if err != nil {
		return internalError("database error")
	}
	user, err := loadWebauthnUser(username, authProvider, methods)
	if err != nil {
		return internalError("failed to load webauthn user")
	}
	credential, err := webauthnRP.ValidateLogin(user, sessionData, parsed)
	if err != nil {
		return badRequest("invalid credential")
	}
	return updateStoredCredential(methods, credential)
}

// updateStoredCredential persists the post-login credential state (sign
// counter, clone flags) back onto the owning method.
func updateStoredCredential(methods []MfaMethod, credential *webauthn.Credential) *apiError {
	for i := range methods {
		if methods[i].Type != mfaTypePasskey || methods[i].Credential == "" {
			continue
		}
		var stored webauthn.Credential
		if err := json.Unmarshal([]byte(methods[i].Credential), &stored); err != nil {
			continue
		}
		if !bytes.Equal(stored.ID, credential.ID) {
			continue
		}
		updatedJSON, err := json.Marshal(credential)
		if err != nil {
			return internalError("failed to encode credential")
		}
		methods[i].Credential = string(updatedJSON)
		if err := db.Save(&methods[i]).Error; err != nil {
			return internalError("failed to update mfa method")
		}
		return nil
	}
	return badRequest("credential does not match an enrolled passkey")
}

func validPhoneNumber(phoneNumber string) bool {
	if len(phoneNumber) < 9 || len(phoneNumber) > 16 || phoneNumber[0] != '+' {
		return false
	}
	for _, c := range phoneNumber[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
