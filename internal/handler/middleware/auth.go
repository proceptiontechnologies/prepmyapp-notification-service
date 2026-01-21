package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims represents the JWT claims structure.
type Claims struct {
	Email    string `json:"email"`
	UserID   string `json:"sub"`
	UserType string `json:"type"`
	jwt.RegisteredClaims
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	JWTSecret string
	APIKeys   []string
}

// JWTAuth creates middleware that validates JWT tokens.
// It extracts the user ID and stores it in the context.
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
			})
			return
		}

		// Check for Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format",
			})
			return
		}

		tokenString := parts[1]

		// Parse and validate the token
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			// Validate signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			return []byte(secret), nil
		})

		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token: " + err.Error(),
			})
			return
		}

		// Extract claims
		claims, ok := token.Claims.(*Claims)
		if !ok || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token claims",
			})
			return
		}

		// Parse user ID as UUID
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid user ID in token",
			})
			return
		}

		// Store user info in context for handlers to use
		c.Set("userID", userID)
		c.Set("email", claims.Email)
		c.Set("userType", claims.UserType)

		c.Next()
	}
}

// APIKeyAuth creates middleware that validates API keys for internal service calls.
func APIKeyAuth(validKeys []string) gin.HandlerFunc {
	// Create a map for O(1) lookup
	keyMap := make(map[string]bool)
	for _, key := range validKeys {
		keyMap[key] = true
	}

	return func(c *gin.Context) {
		// Check X-API-Key header
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing API key",
			})
			return
		}

		// Validate the key
		if !keyMap[apiKey] {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
			})
			return
		}

		c.Next()
	}
}

// OptionalJWTAuth validates JWT if present, but doesn't require it.
// Useful for endpoints that work with or without authentication.
func OptionalJWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.Next()
			return
		}

		tokenString := parts[1]

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.Next()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.Next()
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.Next()
			return
		}

		c.Set("userID", userID)
		c.Set("email", claims.Email)
		c.Set("userType", claims.UserType)

		c.Next()
	}
}

// GetUserID extracts the user ID from the context.
// Returns uuid.Nil if not authenticated.
func GetUserID(c *gin.Context) uuid.UUID {
	userID, exists := c.Get("userID")
	if !exists {
		return uuid.Nil
	}
	id, ok := userID.(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}

// GetEmail extracts the email from the context.
func GetEmail(c *gin.Context) string {
	email, exists := c.Get("email")
	if !exists {
		return ""
	}
	e, ok := email.(string)
	if !ok {
		return ""
	}
	return e
}
