package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"ai-localbase/internal/auth"
	"ai-localbase/internal/service"

	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type tokenResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
	User         userResponse `json:"user"`
}

const (
	minUsernameLen = 3
	maxUsernameLen = 32
	minPasswordLen = 6
)

// Register creates a new user account.
// POST /api/auth/register
func (h *AppHandler) Register(c *gin.Context) {
	if h.userStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth disabled"})
		return
	}
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	username := strings.TrimSpace(req.Username)
	if len(username) < minUsernameLen || len(username) > maxUsernameLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must be 3-32 chars"})
		return
	}
	if len(req.Password) < minPasswordLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 6 chars"})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash password failed"})
		return
	}
	user, err := h.userStore.CreateUser(username, hash)
	if err != nil {
		if errors.Is(err, service.ErrUsernameTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create user failed"})
		return
	}
	c.JSON(http.StatusCreated, userResponse{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	})
}

// Login verifies credentials and returns access + refresh tokens.
// POST /api/auth/login
func (h *AppHandler) Login(c *gin.Context) {
	if h.userStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth disabled"})
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	user, err := h.userStore.GetUserByUsername(strings.TrimSpace(req.Username))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	resp, err := h.issueTokens(user.ID, user.Username, user.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue tokens failed"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Refresh exchanges a valid refresh token for a new token pair (rotation).
// POST /api/auth/refresh
func (h *AppHandler) Refresh(c *gin.Context) {
	if h.userStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth disabled"})
		return
	}
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}
	hash := auth.HashRefreshToken(strings.TrimSpace(req.RefreshToken))
	_, user, err := h.userStore.GetSessionByRefreshTokenHash(hash)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}
	// Rotate: invalidate the old refresh token before issuing a new pair.
	_ = h.userStore.DeleteSessionByHash(hash)
	resp, err := h.issueTokens(user.ID, user.Username, user.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue tokens failed"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Logout invalidates the caller's refresh token. Requires a valid access token.
// POST /api/auth/logout
func (h *AppHandler) Logout(c *gin.Context) {
	if h.userStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth disabled"})
		return
	}
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err == nil && strings.TrimSpace(req.RefreshToken) != "" {
		hash := auth.HashRefreshToken(strings.TrimSpace(req.RefreshToken))
		_ = h.userStore.DeleteSessionByHash(hash)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Me returns the currently authenticated user's profile.
// GET /api/auth/me  (requires Authorization: Bearer <access_token>)
func (h *AppHandler) Me(c *gin.Context) {
	if h.userStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth disabled"})
		return
	}
	userID := auth.UserIDFrom(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	user, err := h.userStore.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, userResponse{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	})
}

// issueTokens signs a new access JWT and persists a fresh refresh-token session.
func (h *AppHandler) issueTokens(userID, username string, createdAt time.Time) (tokenResponse, error) {
	accessTTL := time.Duration(h.serverConfig.AccessTokenTTLMinutes) * time.Minute
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	refreshTTL := time.Duration(h.serverConfig.RefreshTokenTTLDays) * 24 * time.Hour
	if refreshTTL <= 0 {
		refreshTTL = 30 * 24 * time.Hour
	}
	access, err := auth.IssueAccessToken(h.serverConfig.JWTSecret, userID, username, accessTTL)
	if err != nil {
		return tokenResponse{}, err
	}
	raw, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return tokenResponse{}, err
	}
	if _, err := h.userStore.CreateSession(userID, hash, refreshTTL); err != nil {
		return tokenResponse{}, err
	}
	return tokenResponse{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int(accessTTL.Seconds()),
		User: userResponse{
			ID:        userID,
			Username:  username,
			CreatedAt: createdAt,
		},
	}, nil
}
