package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	mfaTypeTotp    = "totp"
	mfaTypeSms     = "sms"
	mfaTypePasskey = "passkey"

	mfaPurposeEnroll = "enroll"
	mfaPurposeVerify = "verify"

	mfaStatusPending   = "pending"
	mfaStatusVerified  = "verified"
	mfaStatusCancelled = "cancelled"

	totpIssuer = "OpenSigner"
	totpPeriod = 30 // seconds, RFC 6238 default

	mfaSessionHeader = "X-MFA-Session"
)

var (
	mfaSessionTTL   = envMinutes("MFA_SESSION_TTL_MINUTES", 15)
	mfaChallengeTTL = envMinutes("MFA_CHALLENGE_TTL_MINUTES", 5)
	mfaMaxAttempts  = envInt("MFA_MAX_ATTEMPTS", 4)

	// trustProxyHeaders must only be enabled when a trusted reverse proxy
	// sets X-Forwarded-For; honoring it from direct clients would let a
	// stolen-JWT attacker forge another device's fingerprint.
	trustProxyHeaders = os.Getenv("TRUST_PROXY_HEADERS") == "true"
)

func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func envMinutes(name string, fallback int) time.Duration {
	return time.Duration(envInt(name, fallback)) * time.Minute
}

// deviceFingerprint derives a stable identifier for the calling device from
// the User-Agent and client IP so a cached MFA session cannot be replayed
// from a different device. Only the hash is ever stored.
func deviceFingerprint(r *http.Request) string {
	sum := sha256.Sum256([]byte(r.Header.Get("User-Agent") + "|" + clientIP(r)))
	return hex.EncodeToString(sum[:])
}

func clientIP(r *http.Request) string {
	if trustProxyHeaders {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			// Take the right-most entry: the address the trusted proxy itself
			// appended. The left-most is client-supplied and spoofable. This
			// assumes exactly one trusted proxy hop.
			parts := strings.Split(fwd, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generateSmsCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("failed to generate code: %w", err)
	}
	return fmt.Sprintf("%06d", n), nil
}

// hashMfaCode keys the hash with the share encryption key so a database
// leak alone does not expose in-flight codes to offline brute force.
func hashMfaCode(code string) string {
	mac := hmac.New(sha256.New, shareEncryptionKey)
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func mfaCodeMatches(code, storedHash string) bool {
	return subtle.ConstantTimeCompare([]byte(hashMfaCode(code)), []byte(storedHash)) == 1
}

func verifiedMfaMethods(username, authProvider string) ([]MfaMethod, error) {
	var methods []MfaMethod
	err := db.Where("username = ? AND auth_provider = ? AND verified = ?", username, authProvider, true).
		Find(&methods).Error
	return methods, err
}

func mfaMethodInfos(methods []MfaMethod) []MfaMethodInfo {
	infos := make([]MfaMethodInfo, len(methods))
	for i, m := range methods {
		infos[i] = MfaMethodInfo{
			ID:        m.ID,
			Type:      m.Type,
			PhoneHint: m.PhoneHint,
			CreatedAt: m.CreatedAt.Unix(),
		}
	}
	return infos
}

// hasValidMfaSession authenticates the request by the opaque token in the
// X-MFA-Session header, not by anything the caller can forge from headers
// alone. Absent or unknown token → no session.
func hasValidMfaSession(r *http.Request, username, authProvider string) bool {
	token := r.Header.Get(mfaSessionHeader)
	if token == "" {
		return false
	}
	var count int64
	db.Model(&MfaSession{}).
		Where("username = ? AND auth_provider = ? AND token_hash = ? AND expires_at > ?",
			username, authProvider, hashSessionToken(token), time.Now()).
		Count(&count)
	return count > 0
}

// createMfaSession issues a new MFA session and returns the opaque token the
// client must present on gated requests. Only the token's hash is stored, so
// a database leak cannot reconstruct a usable session. Expired sessions for
// the user are cleaned up opportunistically.
func createMfaSession(r *http.Request, username, authProvider string) (string, *MfaSession, error) {
	db.Where("username = ? AND auth_provider = ? AND expires_at <= ?",
		username, authProvider, time.Now()).Delete(&MfaSession{})

	token, err := generateSessionToken()
	if err != nil {
		return "", nil, err
	}
	session := MfaSession{
		ID:           uuid.NewString(),
		Username:     username,
		AuthProvider: authProvider,
		TokenHash:    hashSessionToken(token),
		Fingerprint:  deviceFingerprint(r),
		ExpiresAt:    time.Now().Add(mfaSessionTTL),
	}
	if err := db.Create(&session).Error; err != nil {
		return "", nil, err
	}
	return token, &session, nil
}

// revokeAllMfaSessions drops every cached MFA session for the user. Called
// when a method is enrolled so every other active session must re-verify
// (cross-session enforcement), and when the last method is unenrolled.
func revokeAllMfaSessions(username, authProvider string) error {
	return db.Where("username = ? AND auth_provider = ?", username, authProvider).
		Delete(&MfaSession{}).Error
}

// activateMfaSession is the common tail of a successful verification:
// revoke on enrollment (so other devices are challenged), then issue a
// session token for the client that just proved possession.
func activateMfaSession(r *http.Request, username, authProvider string, enrollment bool) (string, *MfaSession, error) {
	if enrollment {
		if err := revokeAllMfaSessions(username, authProvider); err != nil {
			return "", nil, err
		}
	}
	return createMfaSession(r, username, authProvider)
}
