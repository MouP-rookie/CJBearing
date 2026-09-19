// 文件说明：运维命令，用于重置默认 admin 账号密码。

package main

import (
	"bearing-inspection/internal/config"
	"bearing-inspection/internal/database"
	"bearing-inspection/internal/service"
	"flag"
	"log"
)

func main() {
	password := flag.String("password", "1234", "new admin password")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := service.ResetAdminPassword(db, *password); err != nil {
		log.Fatal(err)
	}
	log.Println("admin password reset")
}
