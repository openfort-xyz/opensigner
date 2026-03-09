package main

import (
	"errors"
	"sync"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

var (
	jwkCache   = make(map[string]keyfunc.Keyfunc)
	jwkCacheMu sync.RWMutex
)

func getOrCreateKeyfunc(jwkURL string) (keyfunc.Keyfunc, error) {
	jwkCacheMu.RLock()
	k, ok := jwkCache[jwkURL]
	jwkCacheMu.RUnlock()
	if ok {
		return k, nil
	}

	jwkCacheMu.Lock()
	defer jwkCacheMu.Unlock()

	// Double-check after acquiring write lock
	if k, ok := jwkCache[jwkURL]; ok {
		return k, nil
	}

	k, err := keyfunc.NewDefault([]string{jwkURL})
	if err != nil {
		return nil, err
	}
	jwkCache[jwkURL] = k
	return k, nil
}

func validate(token, jwkURL string) (string, error) {
	k, err := getOrCreateKeyfunc(jwkURL)
	if err != nil {
		return "", err
	}

	parsed, err := jwt.Parse(token, k.Keyfunc, jwt.WithValidMethods([]string{"EdDSA", "RS256", "ES256"}))
	if err != nil {
		return "", errors.New("invalid token")
	}

	claims := parsed.Claims.(jwt.MapClaims)
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errors.New("missing sub claim")
	}
	return sub, nil
}
