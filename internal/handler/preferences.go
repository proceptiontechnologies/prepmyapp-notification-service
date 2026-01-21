package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/prepmyapp/notification/internal/domain"
	"github.com/prepmyapp/notification/internal/handler/middleware"
)

// PreferencesHandler handles notification preferences HTTP requests.
type PreferencesHandler struct {
	repo domain.PreferencesRepository
}

// NewPreferencesHandler creates a new preferences handler.
func NewPreferencesHandler(repo domain.PreferencesRepository) *PreferencesHandler {
	return &PreferencesHandler{repo: repo}
}

// PreferencesResponse represents the user's notification preferences.
type PreferencesResponse struct {
	EmailEnabled    bool            `json:"email_enabled"`
	PushEnabled     bool            `json:"push_enabled"`
	ChannelSettings map[string]bool `json:"channel_settings"`
	QuietHours      *QuietHours     `json:"quiet_hours,omitempty"`
}

// QuietHours represents the quiet hours period.
type QuietHours struct {
	Start string `json:"start"` // Format: "HH:MM"
	End   string `json:"end"`   // Format: "HH:MM"
}

// UpdatePreferencesRequest represents a request to update preferences.
type UpdatePreferencesRequest struct {
	EmailEnabled    *bool           `json:"email_enabled,omitempty"`
	PushEnabled     *bool           `json:"push_enabled,omitempty"`
	ChannelSettings map[string]bool `json:"channel_settings,omitempty"`
	QuietHours      *QuietHours     `json:"quiet_hours,omitempty"`
}

// Get retrieves the current user's notification preferences.
func (h *PreferencesHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	prefs, err := h.repo.Get(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get preferences"})
		return
	}

	response := PreferencesResponse{
		EmailEnabled:    prefs.EmailEnabled,
		PushEnabled:     prefs.PushEnabled,
		ChannelSettings: prefs.ChannelSettings,
	}

	if prefs.QuietHoursStart != nil && prefs.QuietHoursEnd != nil {
		response.QuietHours = &QuietHours{
			Start: prefs.QuietHoursStart.Format("15:04"),
			End:   prefs.QuietHoursEnd.Format("15:04"),
		}
	}

	c.JSON(http.StatusOK, response)
}

// Update updates the current user's notification preferences.
func (h *PreferencesHandler) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current preferences
	prefs, err := h.repo.Get(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get preferences"})
		return
	}

	// Update fields if provided
	if req.EmailEnabled != nil {
		prefs.EmailEnabled = *req.EmailEnabled
	}
	if req.PushEnabled != nil {
		prefs.PushEnabled = *req.PushEnabled
	}
	if req.ChannelSettings != nil {
		if prefs.ChannelSettings == nil {
			prefs.ChannelSettings = make(map[string]bool)
		}
		for channel, enabled := range req.ChannelSettings {
			prefs.ChannelSettings[channel] = enabled
		}
	}

	// Handle quiet hours
	if req.QuietHours != nil {
		if req.QuietHours.Start != "" && req.QuietHours.End != "" {
			startTime, err := time.Parse("15:04", req.QuietHours.Start)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quiet hours start format, use HH:MM"})
				return
			}
			endTime, err := time.Parse("15:04", req.QuietHours.End)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quiet hours end format, use HH:MM"})
				return
			}
			prefs.QuietHoursStart = &startTime
			prefs.QuietHoursEnd = &endTime
		} else {
			// Clear quiet hours if empty strings
			prefs.QuietHoursStart = nil
			prefs.QuietHoursEnd = nil
		}
	}

	// Save updated preferences
	if err := h.repo.Upsert(c.Request.Context(), prefs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update preferences"})
		return
	}

	// Return updated preferences
	response := PreferencesResponse{
		EmailEnabled:    prefs.EmailEnabled,
		PushEnabled:     prefs.PushEnabled,
		ChannelSettings: prefs.ChannelSettings,
	}

	if prefs.QuietHoursStart != nil && prefs.QuietHoursEnd != nil {
		response.QuietHours = &QuietHours{
			Start: prefs.QuietHoursStart.Format("15:04"),
			End:   prefs.QuietHoursEnd.Format("15:04"),
		}
	}

	c.JSON(http.StatusOK, response)
}

// RegisterRoutes registers preferences routes on a router group.
func (h *PreferencesHandler) RegisterRoutes(rg *gin.RouterGroup) {
	prefs := rg.Group("/preferences")
	{
		prefs.GET("", h.Get)
		prefs.PUT("", h.Update)
		prefs.PATCH("", h.Update)
	}
}
