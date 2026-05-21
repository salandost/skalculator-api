package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Secret key for signing JWT
// export JWT secret for use by handlers in other packages
var JwtSecret []byte

// JWTClaims represents the JWT claims
type JWTClaims struct {
	UserID   uint     `json:"sub"`
	Username string   `json:"username"`
	Role     UserRole `json:"role"`
	jwt.RegisteredClaims
}

// JWTAuthMiddleware validates JWT token and extracts user info
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			ResponseJSON(c, http.StatusUnauthorized, "Authorization token required", nil)
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>" format
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader && !strings.Contains(authHeader, "Bearer") {
			// No "Bearer " prefix, use as-is
			tokenString = authHeader
		}

		// Ensure secret is loaded
		if len(JwtSecret) == 0 {
			JwtSecret = []byte(os.Getenv("SECRET_TOKEN"))
		}

		// Parse and validate the token
		token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			// Validate the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return JwtSecret, nil
		})

		if err != nil {
			ResponseJSON(c, http.StatusUnauthorized, "Invalid token", nil)
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*JWTClaims)
		if !ok || !token.Valid {
			ResponseJSON(c, http.StatusUnauthorized, "Invalid token claims", nil)
			c.Abort()
			return
		}

		// Store user info in context for handlers to access
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// RoleAuthMiddleware checks if user has required role(s)
func RoleAuthMiddleware(requiredRoles ...UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			ResponseJSON(c, http.StatusForbidden, "User role not found", nil)
			c.Abort()
			return
		}

		userRole, ok := role.(UserRole)
		if !ok {
			ResponseJSON(c, http.StatusForbidden, "Invalid user role", nil)
			c.Abort()
			return
		}

		// Check if user role is in required roles
		hasRole := false
		for _, req := range requiredRoles {
			if userRole == req {
				hasRole = true
				break
			}
		}

		if !hasRole {
			ResponseJSON(c, http.StatusForbidden, "Insufficient permissions", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}
