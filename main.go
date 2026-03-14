package main

import (
	"log"
	"webhooktester/internal/app"
)

func main() {
	cfg := app.NewConfigFromEnv()
	myApp, err := app.NewApp(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := myApp.SetupDemoUser(); err != nil {
		log.Fatal(err)
	}
	log.Fatal(myApp.Run(":8080")) // TODO: Implement Run method in internal/app
}
