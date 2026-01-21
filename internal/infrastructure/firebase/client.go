package firebase

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"google.golang.org/api/option"

	"github.com/prepmyapp/notification/internal/domain"
)

// Client wraps the Firebase Cloud Messaging client.
type Client struct {
	messaging       *messaging.Client
	deviceTokenRepo domain.DeviceTokenRepository
}

// Config holds Firebase configuration.
type Config struct {
	CredentialsPath string
}

// NewClient creates a new Firebase messaging client.
func NewClient(ctx context.Context, cfg Config, deviceTokenRepo domain.DeviceTokenRepository) (*Client, error) {
	var app *firebase.App
	var err error

	if cfg.CredentialsPath != "" {
		opt := option.WithCredentialsFile(cfg.CredentialsPath)
		app, err = firebase.NewApp(ctx, nil, opt)
	} else {
		// Use default credentials (GCP environment)
		app, err = firebase.NewApp(ctx, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize firebase app: %w", err)
	}

	messagingClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get messaging client: %w", err)
	}

	return &Client{
		messaging:       messagingClient,
		deviceTokenRepo: deviceTokenRepo,
	}, nil
}

// Send sends a push notification to a specific device token.
func (c *Client) Send(ctx context.Context, token, title, body string, data map[string]interface{}) error {
	// Convert data to string map
	stringData := make(map[string]string)
	for k, v := range data {
		stringData[k] = fmt.Sprintf("%v", v)
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: stringData,
		// Android specific config
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ClickAction: "OPEN_NOTIFICATION",
				Sound:       "default",
			},
		},
		// iOS specific config
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound:            "default",
					ContentAvailable: true,
				},
			},
		},
		// Web push config
		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: title,
				Body:  body,
				Icon:  "/icon.png",
			},
		},
	}

	_, err := c.messaging.Send(ctx, message)
	if err != nil {
		// Check if token is invalid and deactivate it
		if messaging.IsUnregistered(err) || messaging.IsInvalidArgument(err) {
			_ = c.deviceTokenRepo.Deactivate(ctx, token)
		}
		return fmt.Errorf("failed to send push notification: %w", err)
	}

	return nil
}

// SendToUser sends a push notification to all of a user's registered devices.
func (c *Client) SendToUser(ctx context.Context, userID uuid.UUID, title, body string, data map[string]interface{}) error {
	// Get user's device tokens
	tokens, err := c.deviceTokenRepo.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get device tokens: %w", err)
	}

	if len(tokens) == 0 {
		return nil // No devices registered, not an error
	}

	// Convert data to string map
	stringData := make(map[string]string)
	for k, v := range data {
		stringData[k] = fmt.Sprintf("%v", v)
	}

	// Build tokens list
	tokenStrings := make([]string, len(tokens))
	for i, t := range tokens {
		tokenStrings[i] = t.Token
	}

	// Send multicast message
	message := &messaging.MulticastMessage{
		Tokens: tokenStrings,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: stringData,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound: "default",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
				},
			},
		},
	}

	response, err := c.messaging.SendEachForMulticast(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send multicast: %w", err)
	}

	// Handle failed tokens
	if response.FailureCount > 0 {
		for i, resp := range response.Responses {
			if !resp.Success {
				// Deactivate invalid tokens
				if messaging.IsUnregistered(resp.Error) || messaging.IsInvalidArgument(resp.Error) {
					_ = c.deviceTokenRepo.Deactivate(ctx, tokenStrings[i])
				}
			}
		}
	}

	return nil
}

// SendToTopic sends a push notification to all subscribers of a topic.
func (c *Client) SendToTopic(ctx context.Context, topic, title, body string, data map[string]interface{}) error {
	stringData := make(map[string]string)
	for k, v := range data {
		stringData[k] = fmt.Sprintf("%v", v)
	}

	message := &messaging.Message{
		Topic: topic,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: stringData,
	}

	_, err := c.messaging.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send topic notification: %w", err)
	}

	return nil
}

// SubscribeToTopic subscribes device tokens to a topic.
func (c *Client) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	_, err := c.messaging.SubscribeToTopic(ctx, tokens, topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}
	return nil
}

// UnsubscribeFromTopic unsubscribes device tokens from a topic.
func (c *Client) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	_, err := c.messaging.UnsubscribeFromTopic(ctx, tokens, topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %w", err)
	}
	return nil
}
