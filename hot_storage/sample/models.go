package main

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type InitEmbeddedRequest struct {
	ChainID int64 `json:"chainId"`
}

type NextAction struct {
	NextAction string    `json:"nextAction"`
	Player     string    `json:"player"` // TODO: rename to user
	Embedded   *Embedded `json:"embedded,omitempty"`
}

type Embedded struct {
	Share        *string `json:"share,omitempty"`
	OwnerAddress *string `json:"ownerAddress,omitempty"`
	Address      *string `json:"address,omitempty"`
	ChainID      int64   `json:"chainId"`
	DeviceID     *string `json:"deviceId,omitempty"`
}

type RegisterEmbeddedRequest struct {
	ChainID    int64   `json:"chainId"`
	Address    string  `json:"address"`
	Share      string  `json:"share"`
	SignerUuid *string `json:"signerUuid"`
}

type ExportedEmbeddedRequest struct {
	Address string `json:"address"`
}

type EmbeddedResponse struct {
	Share        string `json:"share"`
	Address      string `json:"address"`
	ChainID      int64  `json:"chainId"`
	DeviceID     string `json:"deviceId"`
	Device       string `json:"device"`
	Account      string `json:"account"`
	OwnerAddress string `json:"ownerAddress"`
	AccountType  string `json:"accountType"`
	Signer       string `json:"signer"`
}

type CreateDeviceRequest struct {
	AccountId string `json:"accountId"`
	Address   string `json:"address"`
	ChainId   int64  `json:"chainId"`
	Share     string `json:"share"`
}

type Device struct {
	gorm.Model
	ID        string `gorm:"primaryKey" json:"id"`
	Share     string `json:"share"`
	IsPrimary bool   `json:"isPrimary"`
	SignerId  string `json:"signerId"`
}

type Signer struct {
	gorm.Model
	ID string `gorm:"primaryKey" json:"id"`
}

type Account struct {
	gorm.Model
	ID           string `gorm:"primaryKey" json:"id"`
	Address      string `gorm:"uniqueIndex:idx_account_address" json:"address"`
	Username     string `json:"username"` // also referred as userId in the code
	ChainId      int64  `json:"chainId"`
	AuthProvider string `json:"auth_provider"`
	SignerId     string `json:"signerId"`
}

type MigratedAccountData struct {
	gorm.Model
	ID              string `gorm:"primaryKey" json:"id"`
	Wallet          string `json:"wallet"`
	FormerOwnerUser string `json:"former_user"` // used as a PRF seed for passkeys at Openfort
}

type DeviceResponse struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	CreatedAt int64  `json:"createdAt"`
	Address   string `json:"address"`
	Share     string `json:"share"`
	IsPrimary bool   `json:"isPrimary"`
}

type DeviceListResponse struct {
	Object string           `json:"object"`
	URL    string           `json:"url"`
	Data   []DeviceResponse `json:"data"`
	Start  int              `json:"start"`
	End    int              `json:"end"`
	Total  int              `json:"total"`
}

type AccountResponse struct {
	ID       string `gorm:"primaryKey" json:"id"`
	Address  string `json:"address"`
	Username string `json:"username"`
	ChainId  int64  `json:"chainId"`
	SignerId string `json:"signerId"`
}

type AccountListResponse struct {
	Object string            `json:"object"`
	URL    string            `json:"url"`
	Data   []AccountResponse `json:"data"`
	Start  int               `json:"start"`
	End    int               `json:"end"`
	Total  int               `json:"total"`
}

type CreateDeviceResponse = DeviceResponse
type GetDeviceResponse = DeviceResponse

type CreateEmbeddedRequestV2 struct {
	AccountType string  `json:"accountType"`
	ChainType   string  `json:"chainType"`
	ChainId     int64   `json:"chainId"`
	Address     string  `json:"address"`
	Share       string  `json:"share"`
	SignerUuid  *string `json:"signerUuid"`
}

type RecoverEmbeddedRequestV2 struct {
	Account string `json:"account"`
}

type RegisterRequestV2 struct {
	Account string `json:"account"`
	Share   string `json:"share"`
}

type GetSignerResponse struct {
	Id string `json:"id"`
}

type RecoverResponseV2 struct {
	Id            string `json:"id"`
	Account       string `json:"account"`
	SignerAddress string `json:"signerAddress"`
	Signer        string `json:"signer"`
	Share         string `json:"share"`
	IsPrimary     bool   `json:"isPrimary"`
	User          string `json:"user"`
}

type SmartAccountData struct {
	ImplementationType    string  `json:"implementationType,omitempty"`
	FactoryAddress        string  `json:"factoryAddress,omitempty"`
	ImplementationAddress string  `json:"implementationAddress,omitempty"`
	Salt                  string  `json:"salt,omitempty"`
	DeployedTx            string  `json:"deployedTx,omitempty"`
	DeployedAt            float64 `json:"deployedAt,omitempty"`
	Active                bool    `json:"active,omitempty"`
}

type ImportShareRequest struct {
	// AccountV2Response fields
	ID           string            `json:"id"`
	Wallet       string            `json:"wallet,omitempty"`
	AccountType  string            `json:"accountType,omitempty"`
	Address      string            `json:"address"`
	OwnerAddress *string           `json:"ownerAddress"`
	ChainType    string            `json:"chainType,omitempty"`
	ChainId      int64             `json:"chainId,omitempty"`
	SmartAccount *SmartAccountData `json:"smartAccount,omitempty"`
	// Export-specific fields
	Share    string `json:"share"`
	SignerId string `json:"signerId,omitempty"`
	UserId   string `json:"userId"`
	// Added by the sample UI
	Username     string `json:"username"`
	AuthProvider string `json:"authProvider"`
}

type ImportShareResponse struct {
	ID       string `json:"id"`
	Wallet   string `json:"wallet"`
	Address  string `json:"address"`
	SignerId string `json:"signerId"`
}

// MfaMethod is an enrolled MFA factor. Enrollment is scoped to the user
// (username + auth provider), not to a signer: once any method is verified,
// hot share retrieval for every signer of that user requires an MFA session.
type MfaMethod struct {
	gorm.Model
	ID           string `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"index:idx_mfa_method_user" json:"username"`
	AuthProvider string `gorm:"index:idx_mfa_method_user" json:"authProvider"`
	Type         string `json:"type"` // totp | sms | passkey
	Verified     bool   `json:"verified"`
	Secret       string `json:"-"` // totp: encrypted TOTP secret; sms: encrypted phone number
	PhoneHint    string `json:"phoneHint"`
	LastUsedStep int64  `json:"-"` // totp: last accepted 30s timestep (replay guard)
	Credential   string `json:"-"` // passkey: webauthn.Credential JSON (public key material)
}

// MfaChallenge is a single in-progress verification attempt against an
// enrolled (or enrolling) method. Challenges expire and carry an attempt
// counter; exhausted or cancelled challenges can never verify.
type MfaChallenge struct {
	gorm.Model
	ID           string    `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"index:idx_mfa_challenge_user" json:"-"`
	AuthProvider string    `gorm:"index:idx_mfa_challenge_user" json:"-"`
	MethodID     string    `json:"methodId"`
	Type         string    `json:"type"`
	Purpose      string    `json:"-"` // enroll | verify
	Status       string    `json:"status"`
	Attempts     int       `json:"-"`
	CodeHash     string    `json:"-"` // sms: HMAC-SHA256 of the delivered code
	WebauthnData string    `json:"-"` // passkey: webauthn.SessionData JSON
	ExpiresAt    time.Time `json:"expiresAt"`
}

// MfaSession is a completed verification, cached so the same device is not
// re-challenged on every request. Scoped to a device fingerprint
// (User-Agent + IP hash) and expires after mfaSessionTTL.
type MfaSession struct {
	gorm.Model
	ID           string    `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"index:idx_mfa_session_user" json:"-"`
	AuthProvider string    `gorm:"index:idx_mfa_session_user" json:"-"`
	Fingerprint  string    `json:"-"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

type MfaEnrollRequest struct {
	Type        string `json:"type"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
}

type MfaEnrollResponse struct {
	MethodID    string          `json:"methodId"`
	Type        string          `json:"type"`
	ChallengeID string          `json:"challengeId,omitempty"`
	Secret      string          `json:"secret,omitempty"`     // totp: base32 secret for manual entry
	OtpauthURL  string          `json:"otpauthUrl,omitempty"` // totp: otpauth:// provisioning URI
	PublicKey   json.RawMessage `json:"publicKey,omitempty"`  // passkey: WebAuthn creation options
}

type MfaEnrollVerifyRequest struct {
	MethodID    string          `json:"methodId"`
	ChallengeID string          `json:"challengeId,omitempty"`
	Code        string          `json:"code,omitempty"`
	Credential  json.RawMessage `json:"credential,omitempty"`
}

type MfaChallengeRequest struct {
	MethodID string `json:"methodId"`
}

type MfaChallengeResponse struct {
	ChallengeID string          `json:"challengeId"`
	Type        string          `json:"type"`
	Status      string          `json:"status"`
	ExpiresAt   int64           `json:"expiresAt"`
	PublicKey   json.RawMessage `json:"publicKey,omitempty"` // passkey: WebAuthn request options
}

type MfaVerifyRequest struct {
	ChallengeID string          `json:"challengeId"`
	Code        string          `json:"code,omitempty"`
	Credential  json.RawMessage `json:"credential,omitempty"`
}

type MfaVerifyResponse struct {
	Verified  bool  `json:"verified"`
	ExpiresAt int64 `json:"expiresAt"` // MFA session expiry, unix seconds
}

type MfaMethodInfo struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	PhoneHint string `json:"phoneHint,omitempty"`
	CreatedAt int64  `json:"createdAt"`
}

type MfaMethodListResponse struct {
	Object string          `json:"object"`
	URL    string          `json:"url"`
	Data   []MfaMethodInfo `json:"data"`
	Total  int             `json:"total"`
}

// MfaRequiredResponse is the 403 body returned by requireMfa so clients can
// render a method picker without an extra round trip.
type MfaRequiredResponse struct {
	Error   string          `json:"error"`
	Methods []MfaMethodInfo `json:"methods"`
}
