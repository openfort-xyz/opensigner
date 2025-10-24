package main

import (
	"errors"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

func validate(token, jwkURL string) (string, error) {
	k, err := keyfunc.NewDefault([]string{jwkURL})
	if err != nil {
		return "", err
	}

	parsed, err := jwt.Parse(token, k.Keyfunc)
	if err != nil {
		return "", errors.New("invalid token")
	}

	claims := parsed.Claims.(jwt.MapClaims)
	return claims["sub"].(string), nil
}
