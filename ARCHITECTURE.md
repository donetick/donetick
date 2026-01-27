# Donetick Architecture Overview

## Tech Stack
- **Language**: Go 1.24
- **Web framework**: Gin
- **ORM**: GORM (SQLite / PostgreSQL / MySQL)
- **DI**: Uber fx
- **Entry point**: `main.go`

## Project Structure

```
main.go                          # fx DI container, server lifecycle
config/config.go                 # All configuration structs, YAML/env loading

internal/
  chore/                         # "Chores" (tasks) - core domain
    handler.go                   #   HTTP handlers
    scheduler.go                 #   Next-due-date calculation
    model/model.go               #   Chore, ChoreHistory models

  user/                          # Users, notification targets, device tokens
    handler.go
    model/model.go               #   User, UserNotificationTarget, UserDeviceToken

  circle/                        # Groups of users ("circles")
    model/model.go               #   Circle, UserCircleDetail (includes notification prefs)

  notifier/                      # Notification subsystem
    notifier.go                  #   Dispatcher - routes to platform-specific sender
    scheduler.go                 #   Background job: polls DB every 3 min, sends pending
    model/model.go               #   Notification, NotificationPlatform enum
    repo/repository.go           #   DB operations for notifications
    service/
      planner.go                 #   Creates Notification records from chore templates
      telegram/telegram.go       #   Telegram bot API sender
      pushover/pushover.go       #   Pushover API sender
      pushbullet/pushbullet.go   #   Pushbullet API sender
      discord/discord.go         #   Discord webhook sender
      fcm/fcm.go                 #   Firebase Cloud Messaging sender

  events/producer.go             # Webhook event queue (async HTTP POST)
  email/sender.go                # Transactional email (verification, etc.)
  auth/                          # JWT auth, OAuth2, Apple sign-in
  mfa/                           # TOTP-based MFA
  realtime/                      # WebSocket / SSE push
  storage/                       # File storage (S3-compatible)
  thing/                         # "Things" - trackable items with state
  label/, project/, device/      # Supporting domain entities
```

## Notification Flow

```
Chore created/updated
  -> planner.GenerateNotifications()
     creates Notification rows in DB (IsSent=false, ScheduledFor=<time>)

Every 3 minutes:
  -> scheduler.loadAndSendNotificationJob()
     queries unsent notifications due in the past 15h
       -> notifier.SendNotification()
          switch on TypeID:
            1 -> Telegram
            2 -> Pushover
            3 -> Webhook
            4 -> Discord
            5 -> FCM
            6 -> Pushbullet
          marks IsSent=true
```

## Adding a New Notification Platform

1. **Add constant** in `internal/notifier/model/model.go` (the `NotificationPlatform` iota)
2. **Create service** in `internal/notifier/service/<name>/` with a `SendNotification(ctx, *NotificationDetails) error` method
3. **Add config struct** in `config/config.go` and wire env override in `configEnvironmentOverrides`
4. **Register in dispatcher** (`internal/notifier/notifier.go`): add field, constructor param, and switch case
5. **Register in DI** (`main.go`): `fx.Provide(...)` for the constructor

## Configuration

Config is loaded from `config/<env>.yaml` (keyed by `DT_ENV`). Environment variables override with prefix `DT_` (e.g. `DT_PUSHBULLET_API_TOKEN`). Legacy `DONETICK_*` env vars are also supported for some fields.
