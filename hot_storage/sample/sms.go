package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// SmsProvider delivers MFA codes over SMS. Swap gateways (Twilio, SMSAPI,
// ...) by implementing this interface and wiring it in initSmsProvider.
type SmsProvider interface {
	SendCode(phoneNumber, code string) error
}

var smsProvider SmsProvider = logSmsProvider{}

func initSmsProvider() error {
	name := os.Getenv("SMS_PROVIDER")
	switch name {
	case "", "log":
		smsProvider = logSmsProvider{}
	case "twilio":
		provider, err := newTwilioProvider()
		if err != nil {
			return err
		}
		smsProvider = provider
	default:
		return fmt.Errorf("unsupported SMS_PROVIDER %q (supported: log, twilio)", name)
	}
	return nil
}

// logSmsProvider writes codes to the server log instead of sending SMS.
// Development only: anyone with log access can read the codes.
type logSmsProvider struct{}

func (logSmsProvider) SendCode(phoneNumber, code string) error {
	slog.Info("MFA SMS code (log provider, development only)",
		slog.String("phoneNumber", phoneNumber), slog.String("code", code))
	return nil
}

// twilioProvider sends codes through the Twilio Messages API.
type twilioProvider struct {
	accountSID string
	authToken  string
	from       string
	client     *http.Client
}

func newTwilioProvider() (*twilioProvider, error) {
	sid := os.Getenv("TWILIO_ACCOUNT_SID")
	token := os.Getenv("TWILIO_AUTH_TOKEN")
	from := os.Getenv("TWILIO_FROM_NUMBER")
	if sid == "" || token == "" || from == "" {
		return nil, fmt.Errorf("TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, and TWILIO_FROM_NUMBER must be set when SMS_PROVIDER=twilio")
	}
	return &twilioProvider{
		accountSID: sid,
		authToken:  token,
		from:       from,
		client:     &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (t *twilioProvider) SendCode(phoneNumber, code string) error {
	form := url.Values{}
	form.Set("To", phoneNumber)
	form.Set("From", t.from)
	form.Set("Body", fmt.Sprintf("Your OpenSigner verification code is %s", code))

	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.accountSID)
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("failed to build twilio request: %w", err)
	}
	req.SetBasicAuth(t.accountSID, t.authToken)
	req.Header.Set(contentTypeHeader, "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("twilio returned %d: %s", resp.StatusCode, body)
	}
	return nil
}
