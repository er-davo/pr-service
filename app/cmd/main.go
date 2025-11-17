package main

import (
	"context"
	"os"
	"os/signal"
	"pr-service/internal/app"
	"pr-service/internal/config"
	"pr-service/internal/database"
	"pr-service/internal/logger"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	configFilePath := os.Getenv("CONFIG_PATH")
	if configFilePath == "" {
		panic("env ConfigPath is empty")
	}
	cfg, err := config.Load(configFilePath)
	if err != nil {
		panic("error on loading config: " + err.Error())
	}

	log := logger.NewLogger(cfg.App.LogLevel)
	defer log.Sync()

	err = database.Migrate(cfg.App.MirgationDir, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("error on migrating database", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	prApp := app.NewPRApp(cfg, log)

	if err := prApp.Run(ctx); err != nil {
		if ctx.Err() != nil {
			log.Info("app stopped by context")
		} else {
			log.Error("app exited with error", zap.Error(err))
		}
	}
}
