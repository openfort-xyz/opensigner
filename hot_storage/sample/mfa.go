package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
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
)

var (
	mfaSessionTTL   = envMinutes("MFA_SESSION_TTL_MINUTES", 15)
	mfaChallengeTTL = envMinutes("MFA_CHALLENGE_TTL_MINUTES", 5)
	mfaMaxAttempts  = envInt("MFA_MAX_ATTEMPTS", 4)
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
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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

func hasValidMfaSession(r *http.Request, username, authProvider string) bool {
	var count int64
	db.Model(&MfaSession{}).
		Where("username = ? AND auth_provider = ? AND fingerprint = ? AND expires_at > ?",
			username, authProvider, deviceFingerprint(r), time.Now()).
		Count(&count)
	return count > 0
}

// createMfaSession grants the calling device an MFA session, replacing any
// previous session for the same fingerprint. Expired sessions for the user
// are cleaned up opportunistically.
func createMfaSession(r *http.Request, username, authProvider string) (*MfaSession, error) {
	fingerprint := deviceFingerprint(r)
	db.Where("username = ? AND auth_provider = ? AND (fingerprint = ? OR expires_at <= ?)",
		username, authProvider, fingerprint, time.Now()).
		Delete(&MfaSession{})

	session := MfaSession{
		ID:           uuid.NewString(),
		Username:     username,
		AuthProvider: authProvider,
		Fingerprint:  fingerprint,
		ExpiresAt:    time.Now().Add(mfaSessionTTL),
	}
	if err := db.Create(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// revokeAllMfaSessions drops every cached MFA session for the user. Called
// when a method is enrolled so every other active session must re-verify
// (cross-session enforcement), and when the last method is unenrolled.
func revokeAllMfaSessions(username, authProvider string) error {
	return db.Where("username = ? AND auth_provider = ?", username, authProvider).
		Delete(&MfaSession{}).Error
}

// activateMfaSession is the common tail of a successful verification:
// revoke on enrollment (so other devices are challenged), then cache a
// session for the device that just proved possession.
func activateMfaSession(r *http.Request, username, authProvider string, enrollment bool) (*MfaSession, error) {
	if enrollment {
		if err := revokeAllMfaSessions(username, authProvider); err != nil {
			return nil, err
		}
	}
	return createMfaSession(r, username, authProvider)
}
