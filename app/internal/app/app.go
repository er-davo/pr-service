package app

import (
	"context"

	"pr-service/internal/api"
	"pr-service/internal/config"
	"pr-service/internal/database"
	"pr-service/internal/handler"
	"pr-service/internal/repository"
	"pr-service/internal/service"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

// PRApp represents the application with its dependencies.
type PRApp struct {
	cfg *config.Config

	db *pgxpool.Pool
	r  *echo.Echo

	log *zap.Logger
}

// New creates a new App instance, initializes database, services, handlers and routes.
func NewPRApp(cfg *config.Config, log *zap.Logger) *PRApp {
	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	r := echo.New()

	retrier := newRepoRetrier(cfg.Retry, isRetryableFunc)

	teamRepo := repository.NewTeamRepository(db, trmpgx.DefaultCtxGetter, retrier)
	userRepo := repository.NewUserRepository(db, trmpgx.DefaultCtxGetter, retrier)
	prRepo := repository.NewPRRepository(db, trmpgx.DefaultCtxGetter, retrier)

	prService := service.NewPRService(
		teamRepo,
		userRepo,
		prRepo,
		manager.Must(trmpgx.NewDefaultFactory(db)),
		log,
	)

	prHandler := handler.NewPRHandler(prService, log)

	api.RegisterHandlers(r, prHandler)

	r.Use(middleware.Recover())

	return &PRApp{
		cfg: cfg,
		db:  db,
		r:   r,
		log: log,
	}
}

// Run starts the HTTP server and waits for context cancellation.
func (a *PRApp) Run(ctx context.Context) error {
	go func() {
		if err := a.r.Start(":" + a.cfg.App.Port); err != nil {
			a.log.Fatal("failed to start server", zap.Error(err))
		}
	}()

	<-ctx.Done()
	return a.Shutdown()
}

// Shutdown closes database connections and other resources.
func (a *PRApp) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.App.ShutdownTimeout)
	defer cancel()

	if err := a.r.Shutdown(ctx); err != nil {
		a.log.Fatal("failed to shutdown server",
			zap.Error(err),
		)
		return err
	}

	a.db.Close()

	return nil
}
