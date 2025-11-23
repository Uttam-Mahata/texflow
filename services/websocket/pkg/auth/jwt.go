package auth

import (
	"crypto/rsa"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// JWTValidator handles JWT token validation
type JWTValidator struct {
	publicKey *rsa.PublicKey
	secret    []byte
	useRSA    bool
}

// Claims represents the JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(publicKeyPath, secret string) (*JWTValidator, error) {
	validator := &JWTValidator{}

	// Try to load RSA public key first
	if publicKeyPath != "" {
		publicKey, err := loadPublicKey(publicKeyPath)
		if err == nil {
			validator.publicKey = publicKey
			validator.useRSA = true
			return validator, nil
		}
	}

	// Fall back to HMAC with secret
	if secret != "" {
		validator.secret = []byte(secret)
		validator.useRSA = false
		return validator, nil
	}

	return nil, fmt.Errorf("either RSA public key or secret must be provided")
}

// ValidateToken validates a token and returns the claims
func (v *JWTValidator) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if v.useRSA {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return v.publicKey, nil
		}

		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// loadPublicKey loads an RSA public key from a file
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return jwt.ParseRSAPublicKeyFromPEM(keyData)
}
