package auth

import (
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	privateKey            *rsa.PrivateKey
	publicKey             *rsa.PublicKey
	secret                []byte
	accessTokenDuration   time.Duration
	refreshTokenDuration  time.Duration
	useRSA                bool
}

// Claims represents the JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(privateKeyPath, publicKeyPath, secret string, accessTokenDuration, refreshTokenDuration time.Duration) (*JWTManager, error) {
	manager := &JWTManager{
		accessTokenDuration:  accessTokenDuration,
		refreshTokenDuration: refreshTokenDuration,
	}

	// Try to load RSA keys first
	if privateKeyPath != "" && publicKeyPath != "" {
		privateKey, err := loadPrivateKey(privateKeyPath)
		if err == nil {
			publicKey, err := loadPublicKey(publicKeyPath)
			if err == nil {
				manager.privateKey = privateKey
				manager.publicKey = publicKey
				manager.useRSA = true
				return manager, nil
			}
		}
	}

	// Fall back to HMAC with secret
	if secret != "" {
		manager.secret = []byte(secret)
		manager.useRSA = false
		return manager, nil
	}

	return nil, fmt.Errorf("either RSA keys or secret must be provided")
}

// GenerateAccessToken generates a new access token
func (m *JWTManager) GenerateAccessToken(userID primitive.ObjectID, username, email string) (string, error) {
	claims := Claims{
		UserID:   userID.Hex(),
		Username: username,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.accessTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	var token *jwt.Token
	if m.useRSA {
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		return token.SignedString(m.privateKey)
	}

	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// GenerateRefreshToken generates a new refresh token
func (m *JWTManager) GenerateRefreshToken(userID primitive.ObjectID) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID.Hex(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.refreshTokenDuration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	var token *jwt.Token
	if m.useRSA {
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		return token.SignedString(m.privateKey)
	}

	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ValidateToken validates a token and returns the claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if m.useRSA {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return m.publicKey, nil
		}

		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// loadPrivateKey loads an RSA private key from a file
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return jwt.ParseRSAPrivateKeyFromPEM(keyData)
}

// loadPublicKey loads an RSA public key from a file
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return jwt.ParseRSAPublicKeyFromPEM(keyData)
}
