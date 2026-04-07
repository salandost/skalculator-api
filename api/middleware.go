package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"os"
)

// Secret key for signing JWT
var jwtSecret = []byte(os.Getenv("SECRET_TOKEN"))

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			ResponseJSON(c, http.StatusUnauthorized, "Authorization token required", nil)
			c.Abort()
			return
		}
		// parse and validate the token
		_, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Validate the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})
		if err != nil {
			ResponseJSON(c, http.StatusUnauthorized, "Invalid token", nil)
			c.Abort()
			return
		}
		// Token is valid, proceed to the next handler
		c.Next()
	}
}
