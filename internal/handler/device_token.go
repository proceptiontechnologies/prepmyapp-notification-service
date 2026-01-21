package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/prepmyapp/notification/internal/domain"
	"github.com/prepmyapp/notification/internal/handler/middleware"
)

// DeviceTokenRepository defines the interface for device token storage.
type DeviceTokenRepository interface {
	Create(ctx context.Context, token *domain.DeviceToken) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.DeviceToken, error)
	GetByToken(ctx context.Context, token string) (*domain.DeviceToken, error)
	Deactivate(ctx context.Context, token string) error
	Delete(ctx context.Context, token string) error
}

// DeviceTokenHandler handles device token registration HTTP requests.
type DeviceTokenHandler struct {
	repo DeviceTokenRepository
}

// NewDeviceTokenHandler creates a new device token handler.
func NewDeviceTokenHandler(repo DeviceTokenRepository) *DeviceTokenHandler {
	return &DeviceTokenHandler{repo: repo}
}

// RegisterRequest represents a device token registration request.
type RegisterRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform" binding:"required,oneof=ios android web"`
}

// RegisterResponse represents a successful registration response.
type RegisterResponse struct {
	Message string              `json:"message"`
	Device  *domain.DeviceToken `json:"device"`
}

// Register registers or updates a device token for push notifications.
func (h *DeviceTokenHandler) Register(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create device token (upsert - updates if token already exists)
	deviceToken := domain.NewDeviceToken(userID, req.Token, req.Platform)

	if err := h.repo.Create(c.Request.Context(), deviceToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register device token"})
		return
	}

	c.JSON(http.StatusOK, RegisterResponse{
		Message: "device token registered successfully",
		Device:  deviceToken,
	})
}

// List returns all active device tokens for the authenticated user.
func (h *DeviceTokenHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	tokens, err := h.repo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch device tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"devices": tokens,
		"count":   len(tokens),
	})
}

// Unregister removes a device token (logout from device).
func (h *DeviceTokenHandler) Unregister(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	// Verify the token belongs to the user
	deviceToken, err := h.repo.GetByToken(c.Request.Context(), token)
	if err != nil {
		if _, ok := err.(*domain.ErrNotFound); ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "device token not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch device token"})
		return
	}

	if deviceToken.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "device token not found"})
		return
	}

	// Delete the token
	if err := h.repo.Delete(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unregister device token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "device token unregistered successfully"})
}

// RegisterRoutes registers device token routes on a router group.
func (h *DeviceTokenHandler) RegisterRoutes(rg *gin.RouterGroup) {
	devices := rg.Group("/device-tokens")
	devices.POST("", h.Register)
	devices.GET("", h.List)
	devices.DELETE("/:token", h.Unregister)
}
