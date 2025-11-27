package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"auth/internal/models"
	"auth/internal/repository"
	"auth/pkg/auth"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles authentication business logic
type AuthService struct {
	userRepo    *repository.UserRepository
	jwtManager  *auth.JWTManager
	redisClient *redis.Client
	logger      *zap.Logger
	bcryptCost  int
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo *repository.UserRepository,
	jwtManager *auth.JWTManager,
	redisClient *redis.Client,
	logger *zap.Logger,
	bcryptCost int,
) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		jwtManager:  jwtManager,
		redisClient: redisClient,
		logger:      logger,
		bcryptCost:  bcryptCost,
	}
}

// Register registers a new user
func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.AuthResponse, error) {
	// Check if user already exists
	_, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err == nil {
		return nil, fmt.Errorf("user with this email already exists")
	}

	_, err = s.userRepo.FindByUsername(ctx, req.Username)
	if err == nil {
		return nil, fmt.Errorf("user with this username already exists")
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.bcryptCost)
	if err != nil {
		s.logger.Error("Failed to hash password", zap.Error(err))
		return nil, fmt.Errorf("failed to create user")
	}

	// Create user
	user := &models.User{
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: string(passwordHash),
		FullName:     req.FullName,
		Preferences: models.UserPreferences{
			DefaultCompiler: "pdflatex",
			Theme:           "light",
		},
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create user", zap.Error(err))
		return nil, err
	}

	s.logger.Info("User registered successfully",
		zap.String("user_id", user.ID.Hex()),
		zap.String("email", user.Email),
	)

	// Generate tokens
	return s.generateAuthResponse(ctx, user)
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	// Find user by email
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		s.logger.Warn("Login attempt with invalid email", zap.String("email", req.Email))
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.logger.Warn("Login attempt with invalid password",
			zap.String("user_id", user.ID.Hex()),
			zap.String("email", user.Email),
		)
		return nil, fmt.Errorf("invalid credentials")
	}

	s.logger.Info("User logged in successfully",
		zap.String("user_id", user.ID.Hex()),
		zap.String("email", user.Email),
	)

	// Generate tokens
	return s.generateAuthResponse(ctx, user)
}

// RefreshToken generates new tokens using a refresh token
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
	// Validate refresh token
	claims, err := s.jwtManager.ValidateToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Check if token is blacklisted
	isBlacklisted, err := s.isTokenBlacklisted(ctx, refreshToken)
	if err != nil {
		s.logger.Error("Failed to check token blacklist", zap.Error(err))
		return nil, fmt.Errorf("internal error")
	}
	if isBlacklisted {
		return nil, fmt.Errorf("token has been revoked")
	}

	// Get user
	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID in token")
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Blacklist old refresh token
	if err := s.blacklistToken(ctx, refreshToken); err != nil {
		s.logger.Error("Failed to blacklist old token", zap.Error(err))
	}

	// Generate new tokens
	return s.generateAuthResponse(ctx, user)
}

// Logout invalidates a user's tokens
func (s *AuthService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	// Blacklist both tokens
	if err := s.blacklistToken(ctx, accessToken); err != nil {
		s.logger.Error("Failed to blacklist access token", zap.Error(err))
		return err
	}

	if err := s.blacklistToken(ctx, refreshToken); err != nil {
		s.logger.Error("Failed to blacklist refresh token", zap.Error(err))
		return err
	}

	return nil
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(ctx context.Context, userID primitive.ObjectID) (*models.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

// generateAuthResponse generates an authentication response with tokens
func (s *AuthService) generateAuthResponse(ctx context.Context, user *models.User) (*models.AuthResponse, error) {
	// Generate access token
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Email)
	if err != nil {
		s.logger.Error("Failed to generate access token", zap.Error(err))
		return nil, fmt.Errorf("failed to generate tokens")
	}

	// Generate refresh token
	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		s.logger.Error("Failed to generate refresh token", zap.Error(err))
		return nil, fmt.Errorf("failed to generate tokens")
	}

	// Store session in Redis
	sessionKey := fmt.Sprintf("session:%s", user.ID.Hex())
	if err := s.redisClient.Set(ctx, sessionKey, accessToken, 24*time.Hour).Err(); err != nil {
		s.logger.Error("Failed to store session", zap.Error(err))
	}

	// Remove password hash from response
	user.PasswordHash = ""

	return &models.AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(15 * 60), // 15 minutes in seconds
	}, nil
}

// blacklistToken adds a token to the blacklist
func (s *AuthService) blacklistToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("blacklist:%s", token)
	// Store in Redis with 7 days expiry (max refresh token lifetime)
	return s.redisClient.Set(ctx, key, "1", 7*24*time.Hour).Err()
}

// isTokenBlacklisted checks if a token is blacklisted
func (s *AuthService) isTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", token)
	result, err := s.redisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}
