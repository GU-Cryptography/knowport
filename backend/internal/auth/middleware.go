package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// CtxUserID is the gin context key for the authenticated user id.
	CtxUserID = "auth.userID"
	// CtxUsername is the gin context key for the authenticated username.
	CtxUsername = "auth.username"
)

// RequireUser builds a gin middleware that validates Authorization: Bearer <jwt>
// and stores user id / username on the context.
func RequireUser(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}
		claims, err := ParseAccessToken(jwtSecret, strings.TrimSpace(parts[1]))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Next()
	}
}

// UserIDFrom returns the authenticated user id stored by RequireUser, or "".
func UserIDFrom(c *gin.Context) string {
	if v, ok := c.Get(CtxUserID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// UsernameFrom returns the authenticated username stored by RequireUser, or "".
func UsernameFrom(c *gin.Context) string {
	if v, ok := c.Get(CtxUsername); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
