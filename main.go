package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"donetick.com/core/config"
	"donetick.com/core/frontend"
	"donetick.com/core/migrations"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"

	auth "donetick.com/core/internal/authorization"
	"donetick.com/core/internal/chore"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/circle"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/database"
	"donetick.com/core/internal/email"
	label "donetick.com/core/internal/label"
	lRepo "donetick.com/core/internal/label/repo"

	notifier "donetick.com/core/internal/notifier"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	mqtt "donetick.com/core/internal/notifier/service/mqtt"
	"donetick.com/core/internal/notifier/service/pushover"
	telegram "donetick.com/core/internal/notifier/service/telegram"
	"donetick.com/core/internal/thing"
	tRepo "donetick.com/core/internal/thing/repo"
	"donetick.com/core/internal/user"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
)

func main() {
	logging.SetConfig(&logging.Config{
		Encoding:    "console",
		Level:       zapcore.Level(zapcore.DebugLevel),
		Development: true,
	})

	app := fx.New(
		fx.Supply(config.LoadConfig()),
		fx.Supply(logging.DefaultLogger().Desugar()),

		// fx.Provide(config.NewConfig),
		fx.Provide(auth.NewAuthMiddleware),

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
		fx.Provide(mqtt.NewMqttNotifier),
		fx.Provide(notifier.NewNotifier),

		// Rate limiter
		fx.Provide(utils.NewRateLimiter),

		// add email sender:
		fx.Provide(email.NewEmailSender),
		// add handlers also
		fx.Provide(newServer),
		fx.Provide(notifier.NewScheduler),

		// things
		fx.Provide(tRepo.NewThingRepository),

		// Labels:
		fx.Provide(lRepo.NewLabelRepository),
		fx.Provide(label.NewHandler),

		fx.Provide(thing.NewWebhook),
		fx.Provide(thing.NewHandler),

		fx.Provide(chore.NewAPI),

		fx.Provide(frontend.NewHandler),

		// fx.Invoke(RunApp),
		fx.Invoke(
			chore.Routes,
			chore.APIs,
			user.Routes,
			circle.Routes,
			thing.Routes,
			thing.Webhooks,
			label.Routes,
			frontend.Routes,

			func(r *gin.Engine) {},
		),
	)

	if err := app.Err(); err != nil {
		log.Fatal(err)
	}

	app.Run()

}

func newServer(lc fx.Lifecycle, cfg *config.Config, db *gorm.DB, notifier *notifier.Scheduler) *gin.Engine {
	gin.SetMode(gin.DebugMode)
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
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("listen: %s\n", err)
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			if err := srv.Shutdown(context.Background()); err != nil {
				log.Fatalf("Server Shutdown: %s", err)
			}
			return nil
		},
	})

	return r
}
