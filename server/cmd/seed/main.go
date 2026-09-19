package main

import (
	"bearing-inspection/internal/config"
	"bearing-inspection/internal/database"
	"bearing-inspection/internal/service"
	"log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := service.Seed(db); err != nil {
		log.Fatal(err)
	}
	log.Println("seed completed")
}
