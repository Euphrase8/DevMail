package main

import (
	"context"
	"flag"
	"github.com/Euphrase8/DevMail/backend/internal/config"
	"github.com/Euphrase8/DevMail/backend/internal/db"
	"log"
	"time"
)

func main() {
	down := flag.Bool("down", false, "Rollback migrations (removes schema/data)")
	steps := flag.Int("steps", 1, "Number of migrations to roll back")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, config.Load().DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if *down {
		err = db.Rollback(ctx, pool, *steps)
	} else {
		err = db.Migrate(ctx, pool)
	}
	if err != nil {
		log.Fatal(err)
	}
}
