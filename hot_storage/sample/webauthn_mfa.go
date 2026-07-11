package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

var webauthnRP *webauthn.WebAuthn

// initWebauthn configures the WebAuthn relying party used for passkey MFA.
// The RP ID must be the domain of the application embedding the iframe, not
// the iframe's: the WebAuthn ceremony runs in the parent app because
// navigator.credentials is unavailable in React Native webviews and
// restricted in cross-origin iframes on WebKit.
func initWebauthn() error {
	rpID := os.Getenv("WEBAUTHN_RP_ID")
	if rpID == "" {
		rpID = "localhost"
	}
	rpName := os.Getenv("WEBAUTHN_RP_NAME")
	if rpName == "" {
		rpName = "OpenSigner"
	}
	rawOrigins := os.Getenv("WEBAUTHN_RP_ORIGINS")
	if rawOrigins == "" {
		rawOrigins = "http://localhost:7051"
	}
	origins := strings.Split(rawOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	rp, err := webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpName,
		RPOrigins:     origins,
	})
	if err != nil {
		return fmt.Errorf("failed to configure webauthn: %w", err)
	}
	webauthnRP = rp
	return nil
}

// mfaWebauthnUser adapts a hot storage user to the webauthn.User interface.
type mfaWebauthnUser struct {
	username     string
	authProvider string
	credentials  []webauthn.Credential
}

// WebAuthnID hashes the user identity to a fixed 32 bytes: the spec caps
// user handles at 64 bytes and forbids leaking identifying data in them.
func (u *mfaWebauthnUser) WebAuthnID() []byte {
	sum := sha256.Sum256([]byte(u.authProvider + ":" + u.username))
	return sum[:]
}

func (u *mfaWebauthnUser) WebAuthnName() string { return u.username }

func (u *mfaWebauthnUser) WebAuthnDisplayName() string { return u.username }

func (u *mfaWebauthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// loadWebauthnUser builds the webauthn.User for a hot storage user from the
// given passkey methods (used for exclusion lists and assertion validation).
func loadWebauthnUser(username, authProvider string, methods []MfaMethod) (*mfaWebauthnUser, error) {
	user := &mfaWebauthnUser{username: username, authProvider: authProvider}
	for _, m := range methods {
		if m.Type != mfaTypePasskey || m.Credential == "" {
			continue
		}
		var cred webauthn.Credential
		if err := json.Unmarshal([]byte(m.Credential), &cred); err != nil {
			return nil, fmt.Errorf("failed to decode stored credential %s: %w", m.ID, err)
		}
		user.credentials = append(user.credentials, cred)
	}
	return user, nil
}

func credentialDescriptors(creds []webauthn.Credential) []protocol.CredentialDescriptor {
	descriptors := make([]protocol.CredentialDescriptor, len(creds))
	for i, c := range creds {
		descriptors[i] = c.Descriptor()
	}
	return descriptors
}
