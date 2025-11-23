package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User represents a user in the system
type User struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email         string             `bson:"email" json:"email"`
	Username      string             `bson:"username" json:"username"`
	PasswordHash  string             `bson:"password_hash" json:"-"`
	FullName      string             `bson:"full_name" json:"full_name"`
	AvatarURL     string             `bson:"avatar_url,omitempty" json:"avatar_url,omitempty"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
	EmailVerified bool               `bson:"email_verified" json:"email_verified"`
	OAuthProviders []OAuthProvider   `bson:"oauth_providers,omitempty" json:"oauth_providers,omitempty"`
	Preferences   UserPreferences    `bson:"preferences" json:"preferences"`
}

// OAuthProvider represents an OAuth provider connection
type OAuthProvider struct {
	Provider       string    `bson:"provider" json:"provider"`
	ProviderUserID string    `bson:"provider_user_id" json:"provider_user_id"`
	ConnectedAt    time.Time `bson:"connected_at" json:"connected_at"`
}

// UserPreferences holds user preferences
type UserPreferences struct {
	DefaultCompiler string `bson:"default_compiler" json:"default_compiler"`
	Theme           string `bson:"theme" json:"theme"`
}

// Session represents a user session
type Session struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID       primitive.ObjectID `bson:"user_id" json:"user_id"`
	RefreshToken string             `bson:"refresh_token" json:"-"`
	ExpiresAt    time.Time          `bson:"expires_at" json:"expires_at"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	IPAddress    string             `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
	UserAgent    string             `bson:"user_agent,omitempty" json:"user_agent,omitempty"`
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=30"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required"`
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse represents the response after successful authentication
type AuthResponse struct {
	User         *User  `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
