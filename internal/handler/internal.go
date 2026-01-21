package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/prepmyapp/notification/internal/domain"
	"github.com/prepmyapp/notification/internal/service"
)

// InternalHandler handles internal API requests from other services.
type InternalHandler struct {
	service *service.NotificationService
}

// NewInternalHandler creates a new internal API handler.
func NewInternalHandler(svc *service.NotificationService) *InternalHandler {
	return &InternalHandler{service: svc}
}

// NotifyRequest represents a request to send a notification.
type NotifyRequest struct {
	UserID   string                 `json:"user_id" binding:"required"`
	Email    string                 `json:"email"`
	Channels []string               `json:"channels" binding:"required"` // ["email", "push", "in_app"] or ["all"]
	Template string                 `json:"template"`
	Title    string                 `json:"title" binding:"required"`
	Body     string                 `json:"body" binding:"required"`
	Data     map[string]interface{} `json:"data"`
}

// NotifyResponse represents the response from a notify request.
type NotifyResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Notify sends a notification through the specified channels.
func (h *InternalHandler) Notify(c *gin.Context) {
	var req NotifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NotifyResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, NotifyResponse{
			Success: false,
			Error:   "invalid user_id format",
		})
		return
	}

	// Parse channels
	channels := h.parseChannels(req.Channels)
	if len(channels) == 0 {
		c.JSON(http.StatusBadRequest, NotifyResponse{
			Success: false,
			Error:   "no valid channels specified",
		})
		return
	}

	// Build send request
	sendReq := service.SendRequest{
		UserID:   userID,
		Email:    req.Email,
		Channels: channels,
		Template: req.Template,
		Title:    req.Title,
		Body:     req.Body,
		Data:     req.Data,
	}

	// Send notification
	if err := h.service.Send(c.Request.Context(), sendReq); err != nil {
		c.JSON(http.StatusInternalServerError, NotifyResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, NotifyResponse{
		Success: true,
		Message: "notification sent successfully",
	})
}

// BulkNotifyRequest represents a request to send notifications to multiple users.
type BulkNotifyRequest struct {
	UserIDs  []string               `json:"user_ids" binding:"required"`
	Emails   map[string]string      `json:"emails"` // userID -> email mapping
	Channels []string               `json:"channels" binding:"required"`
	Template string                 `json:"template"`
	Title    string                 `json:"title" binding:"required"`
	Body     string                 `json:"body" binding:"required"`
	Data     map[string]interface{} `json:"data"`
}

// BulkNotifyResponse represents the response from a bulk notify request.
type BulkNotifyResponse struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

// NotifyBulk sends notifications to multiple users.
func (h *InternalHandler) NotifyBulk(c *gin.Context) {
	var req BulkNotifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	channels := h.parseChannels(req.Channels)
	if len(channels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid channels specified"})
		return
	}

	var (
		success int
		failed  int
		errors  []string
	)

	for _, userIDStr := range req.UserIDs {
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			failed++
			errors = append(errors, "invalid user_id: "+userIDStr)
			continue
		}

		email := ""
		if req.Emails != nil {
			email = req.Emails[userIDStr]
		}

		sendReq := service.SendRequest{
			UserID:   userID,
			Email:    email,
			Channels: channels,
			Template: req.Template,
			Title:    req.Title,
			Body:     req.Body,
			Data:     req.Data,
		}

		if err := h.service.Send(c.Request.Context(), sendReq); err != nil {
			failed++
			errors = append(errors, userIDStr+": "+err.Error())
		} else {
			success++
		}
	}

	c.JSON(http.StatusOK, BulkNotifyResponse{
		Success: success,
		Failed:  failed,
		Errors:  errors,
	})
}

// parseChannels converts channel strings to NotificationType.
func (h *InternalHandler) parseChannels(channels []string) []domain.NotificationType {
	var result []domain.NotificationType

	for _, ch := range channels {
		switch ch {
		case "all":
			return []domain.NotificationType{
				domain.NotificationTypeEmail,
				domain.NotificationTypePush,
				domain.NotificationTypeInApp,
			}
		case "email":
			result = append(result, domain.NotificationTypeEmail)
		case "push":
			result = append(result, domain.NotificationTypePush)
		case "in_app":
			result = append(result, domain.NotificationTypeInApp)
		}
	}

	return result
}

// RegisterRoutes registers internal API routes.
func (h *InternalHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/notify", h.Notify)
	rg.POST("/notify/bulk", h.NotifyBulk)
}
