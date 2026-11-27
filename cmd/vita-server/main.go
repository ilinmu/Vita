package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ilinmu/vita/internal/server"
)

func envOrDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func main() {
	addr := flag.String("addr", envOrDefault("VITA_ADDR", ":8420"), "HTTP listen address")
	dataDir := flag.String("data", envOrDefault("VITA_DATA_DIR", "./data"), "SQLite & secret data directory")
	allowReg := flag.Bool("allow-register", envOrDefault("VITA_ALLOW_PUBLIC_REGISTER", "true") != "false", "Allow public registration without invite code")
	flag.Parse()

	dbPath := filepath.Join(*dataDir, "vita-server.db")
	db, err := server.OpenDB(dbPath)
	if err != nil {
		log.Fatalf("open sqlite db %s: %v", dbPath, err)
	}
	defer db.Close()

	secret, err := server.LoadOrCreateJWTSecret(*dataDir, os.Getenv("VITA_JWT_SECRET"))
	if err != nil {
		log.Fatalf("load jwt secret: %v", err)
	}

	srv := server.New(db, server.Options{
		JWTSecret:           secret,
		AllowPublicRegister: *allowReg,
		Version:             "0.2.0",
	})

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("vita-server listening on %s (data=%s, allow_public_register=%v)", *addr, *dataDir, *allowReg)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down vita-server...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutCtx)
}
