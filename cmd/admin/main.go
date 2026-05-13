package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/brian-nunez/video-to-blog-page/internal/auth"
	"github.com/brian-nunez/video-to-blog-page/internal/config"
	"github.com/brian-nunez/video-to-blog-page/internal/db"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "create-user":
		createUser(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func createUser(args []string) {
	fs := flag.NewFlagSet("create-user", flag.ExitOnError)
	var username string
	var password string
	fs.StringVar(&username, "username", "", "admin username")
	fs.StringVar(&password, "password", "", "admin password (min 8 chars)")
	_ = fs.Parse(args)

	if username == "" || password == "" {
		log.Fatal("username and password are required")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	ctx := context.Background()
	store, err := db.Open(ctx, cfg.SQLitePath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()
	if err := store.RunDefaultMigrations(ctx); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	authSvc := auth.Service{
		Store:      store,
		SessionTTL: 7 * 24 * time.Hour,
		CookieName: "vtb_admin_session",
	}
	user, err := authSvc.CreateAdminUser(ctx, username, password)
	if err != nil {
		log.Fatalf("create admin user: %v", err)
	}
	fmt.Printf("created admin user: %s (%s)\n", user.Username, user.ID)
}

func usage() {
	fmt.Println(`Usage:
  go run ./cmd/admin create-user --username admin --password 'change-me'`)
}
