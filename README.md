# PrepMyApp Notification Service

A Go-based notification microservice for the PrepMyApp platform. Handles push notifications, email notifications, real-time WebSocket updates, and user notification preferences.

## Features

- **Push Notifications**: Firebase Cloud Messaging (FCM) integration for mobile push notifications
- **Email Notifications**: SendGrid integration for transactional emails
- **Real-time Updates**: WebSocket support for instant in-app notifications
- **Notification Preferences**: User-configurable notification settings
- **Device Token Management**: Register and manage mobile device tokens
- **JWT Authentication**: Secure API access with JWT tokens
- **API Key Authentication**: Internal service-to-service communication

## Tech Stack

- **Language**: Go 1.24+
- **Framework**: Gin (HTTP router)
- **Database**: PostgreSQL
- **Push Notifications**: Firebase Cloud Messaging
- **Email**: SendGrid
- **Real-time**: WebSocket (gorilla/websocket)

## Getting Started

### Prerequisites

- Go 1.24 or higher
- PostgreSQL database
- Firebase project with Cloud Messaging enabled
- SendGrid account (for email notifications)

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/your-org/prepmyapp-notification-service.git
   cd prepmyapp-notification-service
   ```

2. Copy the environment file and configure:
   ```bash
   cp .env.example .env
   ```

3. Set up your environment variables in `.env`:
   ```env
   PORT=5003
   ENVIRONMENT=development
   DATABASE_URL=postgres://user:pass@localhost:5433/prepmyapp?sslmode=disable
   JWT_SECRET=your-jwt-secret
   SENDGRID_API_KEY=your-sendgrid-key
   FIREBASE_CREDENTIALS_PATH=./firebase-credentials.json
   INTERNAL_API_KEYS=your-api-key
   ```

4. Add your Firebase credentials:
   - Download your Firebase service account JSON from Firebase Console
   - Save it as `firebase-credentials.json` in the project root

5. Run database migrations:
   ```bash
   make migrate-up
   ```

6. Start the server:
   ```bash
   make run
   ```

### Development

For hot-reload during development:
```bash
make dev
```

This uses [Air](https://github.com/cosmtrek/air) for live reloading.

## API Endpoints

### Health Check
- `GET /health` - Service health status
- `GET /ready` - Readiness check (includes database)

### Public API (JWT Auth Required)
- `GET /api/v1/notifications` - List user notifications
- `POST /api/v1/notifications/read` - Mark notifications as read
- `GET /api/v1/preferences` - Get notification preferences
- `PUT /api/v1/preferences` - Update notification preferences
- `POST /api/v1/devices` - Register device token
- `DELETE /api/v1/devices/:token` - Remove device token

### Internal API (API Key Auth Required)
- `POST /internal/v1/notifications` - Send notification (from backend services)
- `POST /internal/v1/notifications/bulk` - Send bulk notifications

### WebSocket
- `GET /ws?token=<jwt>` - WebSocket connection for real-time updates

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `5003` |
| `ENVIRONMENT` | `development` or `production` | `development` |
| `DATABASE_URL` | PostgreSQL connection string | - |
| `JWT_SECRET` | Secret for JWT validation | - |
| `SENDGRID_API_KEY` | SendGrid API key | - |
| `SENDGRID_FROM_EMAIL` | Sender email address | - |
| `SENDGRID_FROM_NAME` | Sender display name | - |
| `FIREBASE_CREDENTIALS_PATH` | Path to Firebase credentials JSON | - |
| `INTERNAL_API_KEYS` | Comma-separated API keys | - |

## Docker

Build and run with Docker:

```bash
docker build -t prepmyapp-notification-service .
docker run -p 5003:5003 --env-file .env prepmyapp-notification-service
```

## Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/              # Configuration loading
│   ├── database/            # Database connection
│   ├── handler/             # HTTP handlers
│   │   └── middleware/      # Auth middleware
│   ├── infrastructure/      # External services
│   │   ├── firebase/        # FCM client
│   │   ├── sendgrid/        # Email client
│   │   └── websocket/       # WebSocket hub
│   ├── repository/          # Data access layer
│   │   └── postgres/        # PostgreSQL repositories
│   └── service/             # Business logic
├── migrations/              # SQL migrations
├── Dockerfile
├── Makefile
└── go.mod
```

## Makefile Commands

```bash
make build      # Build the binary
make run        # Run the server
make dev        # Run with hot-reload
make test       # Run tests
make lint       # Run linter
make migrate-up # Run migrations
make migrate-down # Rollback migrations
```

## License

MIT
