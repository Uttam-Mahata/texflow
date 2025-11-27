package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"collaboration/pkg/auth"
	"go.uber.org/zap"
)

// AuthMiddleware validates JWT tokens for WebSocket connections
func AuthMiddleware(jwtValidator *auth.JWTValidator, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from query parameter (for WebSocket upgrade) or header
		token := c.Query("token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					token = parts[1]
				}
			}
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No token provided"})
			c.Abort()
			return
		}

		// Validate token
		claims, err := jwtValidator.ValidateToken(token)
		if err != nil {
			logger.Warn("Invalid token", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)

		c.Next()
	}
}
