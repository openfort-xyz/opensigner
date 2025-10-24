package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

const (
	headerAuth            = "Authorization"
	headerAuthPrefix      = "Bearer "
	headerAuthProvider    = "X-Auth-Provider"
	headerCookieFieldName = "X-Cookie-Field"

	auhtProviderDefault = "default"
	authProviderGoogle  = "google"
	authProviderPlayFab = "playfab"

	authProviderGoogleUrl = "https://www.googleapis.com/oauth2/v3/certs"
)

var (
	ErrDuplicateToken = errors.New("only one token delivery method (header or cookie) is allowed")
	ErrMissingToken   = errors.New("missing token")
	ErrInvalidToken   = errors.New("invalid token")
)

func validateAuth(r *http.Request) (string, string, error) {
	token, err := getToken(r)
	if err != nil {
		return "", "", err
	}

	authProvider := r.Header.Get(headerAuthProvider)
	if authProvider == "" {
		authProvider = auhtProviderDefault
	}

	var userId string
	switch authProvider {
	case auhtProviderDefault:
		userId, err = validateDefaultAuth(token)
	default:
		userId, err = validateThirdPartyAuth(token, authProvider)
	}
	return userId, authProvider, err
}

func validateThirdPartyAuth(token string, authProvider string) (string, error) {
	switch authProvider {
	case authProviderGoogle:
		return validate(token, authProviderGoogleUrl)
	case authProviderPlayFab:
		return "", errors.New("playfab third party authentication is unimplemented")
	default:
		return "", errors.New("unsupported auth provider")
	}
}

func validateDefaultAuth(token string) (string, error) {
	jwkUrl := fmt.Sprintf("%s/.well-known/jwks.json", authServerURL)
	userId, err := validate(token, jwkUrl)
	if err != nil {
		slog.Info(fmt.Sprintf("failed to authenticate user: '%v'", err))
		return "", err
	}
	slog.Info("authenticated user", slog.String("externalUserId", userId))
	return userId, nil
}

func unauthorized(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized"}`))
}

func getToken(r *http.Request) (string, error) {
	headerToken, err := getTokenFromHeader(r)
	if err != nil && !errors.Is(err, ErrMissingToken) {
		return "", err
	}

	cookieToken, err := getTokenFromCookie(r)
	if err != nil && !errors.Is(err, ErrMissingToken) {
		return "", err
	}

	if cookieToken != "" && headerToken != "" {
		return "", ErrDuplicateToken
	}

	if cookieToken == "" && headerToken == "" {
		return "", ErrMissingToken
	}

	var token string
	if headerToken != "" {
		token = headerToken
	} else {
		token = cookieToken
	}
	return token, nil
}

func getTokenFromHeader(r *http.Request) (string, error) {
	token := r.Header.Get(headerAuth)
	token = strings.TrimPrefix(token, headerAuthPrefix)
	if token == "" {
		return "", ErrMissingToken
	}
	return token, nil
}

func getTokenFromCookie(r *http.Request) (string, error) {
	cookieFieldName := r.Header.Get(headerCookieFieldName)
	if cookieFieldName == "" {
		return "", ErrMissingToken
	}

	cookie, err := r.Cookie(cookieFieldName)
	if err != nil {
		if err == http.ErrNoCookie {
			return "", ErrMissingToken
		}
		return "", ErrInvalidToken
	}
	token := cookie.Value
	if token == "" {
		return "", ErrMissingToken
	}
	return token, nil
}
