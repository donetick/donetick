package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"donetick.com/core/config"
	"donetick.com/core/frontend"
	"donetick.com/core/migrations"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"gorm.io/gorm"

	auth "donetick.com/core/internal/authorization"
	"donetick.com/core/internal/chore"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/circle"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/database"
	"donetick.com/core/internal/email"
	"donetick.com/core/internal/events"
	label "donetick.com/core/internal/label"
	lRepo "donetick.com/core/internal/label/repo"
	"donetick.com/core/internal/mfa"
	"donetick.com/core/internal/resource"
	"donetick.com/core/internal/storage"
	storageRepo "donetick.com/core/internal/storage/repo"
	spRepo "donetick.com/core/internal/subtask/repo"

	notifier "donetick.com/core/internal/notifier"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	discord "donetick.com/core/internal/notifier/service/discord"
	"donetick.com/core/internal/notifier/service/pushover"
	telegram "donetick.com/core/internal/notifier/service/telegram"
	pRepo "donetick.com/core/internal/points/repo"
	"donetick.com/core/internal/thing"
	tRepo "donetick.com/core/internal/thing/repo"
	"donetick.com/core/internal/user"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
)

func main() {
	// Load configuration first
	cfg := config.LoadConfig()

	// Configure logging from application config
	logging.SetConfigFromAppConfig(
		cfg.Logging.Level,
		cfg.Logging.Encoding,
		cfg.Logging.Development,
	)

	app := fx.New(
		fx.Supply(cfg),
		fx.Supply(logging.DefaultLogger().Desugar()),

		// fx.Provide(config.NewConfig),
		fx.Provide(auth.NewAuthMiddleware),
		fx.Provide(auth.NewIdentityProvider),
		fx.Provide(resource.NewHandler),

		// fx.Provide(NewBot),
		fx.Provide(database.NewDatabase),
		fx.Provide(chRepo.NewChoreRepository),
		fx.Provide(chore.NewHandler),
		fx.Provide(uRepo.NewUserRepository),
		fx.Provide(user.NewHandler),
		fx.Provide(cRepo.NewCircleRepository),
		fx.Provide(circle.NewHandler),

		fx.Provide(nRepo.NewNotificationRepository),
		fx.Provide(nps.NewNotificationPlanner),

		// add notifier
		fx.Provide(pushover.NewPushover),
		fx.Provide(telegram.NewTelegramNotifier),
		fx.Provide(discord.NewDiscordNotifier),
		fx.Provide(notifier.NewNotifier),
		fx.Provide(events.NewEventsProducer),

		// Rate limiter
		fx.Provide(utils.NewRateLimiter),

		// add email sender:
		fx.Provide(email.NewEmailSender),

		// MFA services
		fx.Provide(mfa.NewService),
		fx.Provide(mfa.NewCleanupService),

		// add handlers also
		fx.Provide(newServer),
		fx.Provide(notifier.NewScheduler),

		// things
		fx.Provide(tRepo.NewThingRepository),

		// points
		fx.Provide(pRepo.NewPointsRepository),
		fx.Provide(spRepo.NewSubTasksRepository),

		// Labels:
		fx.Provide(lRepo.NewLabelRepository),
		fx.Provide(label.NewHandler),

		fx.Provide(thing.NewAPI),
		fx.Provide(thing.NewHandler),

		fx.Provide(chore.NewAPI),

		fx.Provide(frontend.NewHandler),

		// storage :
		// is storage local or remote?
		// fx.Provide(storage.NewLocalStorage),
		// fx.Provide(storage.NewURLSignerLocal),
		fx.Provide(storage.NewS3Storage),
		fx.Provide(storage.NewURLSignerS3),

		fx.Provide(storage.NewHandler),
		fx.Provide(storageRepo.NewStorageRepository),

		// fx.Invoke(RunApp),
		fx.Invoke(
			chore.Routes,
			chore.APIs,
			user.Routes,
			circle.Routes,
			thing.Routes,
			thing.APIs,
			label.Routes,
			storage.Routes,
			frontend.Routes,
			resource.Routes,

			func(r *gin.Engine) {},
		),
	)

	if err := app.Err(); err != nil {
		log.Fatal(err)
	}

	app.Run()

}

func newServer(lc fx.Lifecycle, cfg *config.Config, db *gorm.DB, notifier *notifier.Scheduler, eventProducer *events.EventsProducer, mfaCleanup *mfa.CleanupService) *gin.Engine {
	// Set Gin mode based on logging configuration
	if cfg.Logging.Development || strings.ToLower(cfg.Logging.Level) == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// log when http request is made:

	r := gin.New()
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	config := cors.DefaultConfig()
	if cfg.IsDoneTickDotCom {
		// config.AllowOrigins = cfg.Server.CorsAllowOrigins
		config.AllowAllOrigins = true
	} else {
		config.AllowAllOrigins = true
	}

	config.AllowCredentials = true
	config.AddAllowHeaders("Authorization", "secretkey")
	r.Use(cors.New(config))

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if cfg.Database.Migration {
				database.Migration(db)
				migrations.Run(context.Background(), db)
				err := database.MigrationScripts(db, cfg)
				if err != nil {
					panic(err)
				}
			}
			notifier.Start(context.Background())
			eventProducer.Start(context.Background())
			mfaCleanup.Start(context.Background())
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("listen: %s\n", err)
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			mfaCleanup.Stop()
			if err := srv.Shutdown(context.Background()); err != nil {
				log.Fatalf("Server Shutdown: %s", err)
			}
			return nil
		},
	})

	return r
}
