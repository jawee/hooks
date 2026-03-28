package main

import (
	"log"
	"log/slog"
	"os"
	"webhooktester/internal/app"
)

func main() {
	h := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(h)
	cfg := app.NewConfigFromEnv()
	myApp, err := app.NewApp(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := myApp.SetupDemoUser(); err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(myApp.Run(":" + port)) // TODO: Implement Run method in internal/app
}
