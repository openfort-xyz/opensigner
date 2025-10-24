package main

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// KeyType represents the supported key types.
type KeyType int8

const (
	KeyTypeUnknown KeyType = iota
	KeyTypeRSA
	KeyTypeECDSA
	KeyTypeEd25519
)

// getKeyFuncFromPEM parses the PEM and returns a jwt.Keyfunc for validation.
func getKeyFuncFromPEM(pem []byte, keyType KeyType) (jwt.Keyfunc, error) {
	var pubKey any
	var err error
	switch keyType {
	case KeyTypeRSA:
		pubKey, err = jwt.ParseRSAPublicKeyFromPEM(pem)
	case KeyTypeECDSA:
		pubKey, err = jwt.ParseECPublicKeyFromPEM(pem)
	case KeyTypeEd25519:
		pubKey, err = jwt.ParseEdPublicKeyFromPEM(pem)
	default:
		return nil, errors.New("unsupported key type")
	}
	if err != nil {
		return nil, err
	}
	return func(token *jwt.Token) (any, error) {
		return pubKey, nil
	}, nil
}

// validatePEM validates a JWT using the provided PEM and key type.
// TODO.
func validatePEM(tokenString string, pem []byte, keyType KeyType) (string, error) {
	keyFunc, err := getKeyFuncFromPEM(pem, keyType)
	if err != nil {
		return "", fmt.Errorf("failed to parse PEM: %w", err)
	}
	parsed, err := jwt.Parse(tokenString, keyFunc)
	if err != nil {
		return "", fmt.Errorf("failed to validate JWT: %w", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return "", errors.New("invalid JWT claims")
	}
	sub, ok := claims["sub"].(string)
	if !ok {
		return "", errors.New("JWT missing 'sub' claim")
	}
	return sub, nil
}
