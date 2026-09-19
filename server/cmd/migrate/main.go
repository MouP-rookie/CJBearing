// 文件说明：数据库迁移命令入口，支持 up、down 和 steps。

package main

import (
	"errors"
	"log"
	"os"
	"strconv"

	"bearing-inspection/internal/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	m, err := migrate.New("file://migrations", cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer m.Close()
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	case "steps":
		if len(os.Args) < 3 {
			log.Fatal("usage: migrate steps <n>")
		}
		n, parseErr := strconv.Atoi(os.Args[2])
		if parseErr != nil {
			log.Fatal(parseErr)
		}
		err = m.Steps(n)
	default:
		log.Fatalf("unknown command: %s", cmd)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatal(err)
	}
	log.Println("migration completed")
}
