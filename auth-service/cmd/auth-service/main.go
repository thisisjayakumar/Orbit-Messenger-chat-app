package main

import (
    "flag"
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

    kconfig "github.com/go-kratos/kratos/v2/config"
    kfile "github.com/go-kratos/kratos/v2/config/file"
    aconf "github.com/thisisjayakumar/Orbit-Messenger-chat-app/auth-service/internal/conf"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/auth-service/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/auth-service/internal/data"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/auth-service/internal/server"
)

func main() {
    var confPath string
    flag.StringVar(&confPath, "conf", "auth-service/configs/config.yaml", "config file path")
    flag.Parse()

    // Load config (YAML) if provided
    var bc aconf.Bootstrap
    if confPath != "" {
        c := kconfig.New(kconfig.WithSource(kfile.NewSource(confPath)))
        if err := c.Load(); err != nil {
            log.Fatal("Failed to load config:", err)
        }
        if err := c.Scan(&bc); err != nil {
            log.Fatal("Failed to parse config:", err)
        }
        defer c.Close()
    }

	// Database connection
    dbSource := getEnv("DATABASE_URL", "")
    if dbSource == "" {
        if bc.Data != nil && bc.Data.Database != nil && bc.Data.Database.Source != "" {
            dbSource = bc.Data.Database.Source
        } else {
            dbSource = "postgres://chat_user:chat_password@localhost:5432/chat_db?sslmode=disable"
        }
    }
    log.Printf("Using database source: %s", dbSource)
    db, err := sql.Open("postgres", dbSource)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Repository
	authRepo := data.NewAuthRepo(db)

	// Use case
    jwtSecret := getEnv("JWT_SECRET", "your-secret-key-change-this-in-production")
	tokenTTL := 24 * time.Hour
	keycloakConfig := biz.KeycloakConfig{
        URL:          getEnv("KEYCLOAK_URL", "http://localhost:8080"),
        Realm:        getEnv("KEYCLOAK_REALM", "orbit-chat"),
        ClientID:     getEnv("KEYCLOAK_CLIENT_ID", "orbit-chat-client"),
        ClientSecret: getEnv("KEYCLOAK_CLIENT_SECRET", "your-client-secret"),
	}
	authUc, err := biz.NewAuthUsecase(authRepo, jwtSecret, tokenTTL, keycloakConfig)
	if err != nil {
		log.Fatal("Failed to create auth usecase:", err)
	}

	// HTTP server
	httpServer := server.NewHTTPServer(authUc)

	// Start server
    listenAddr := ":" + getEnv("PORT", "")
    if listenAddr == ":" {
        if bc.Server != nil && bc.Server.Http != nil && bc.Server.Http.Addr != "" {
            listenAddr = bc.Server.Http.Addr
        } else {
            listenAddr = ":8000"
        }
    }
    srv := &http.Server{Addr: listenAddr, Handler: httpServer}

	go func() {
        log.Printf("Auth service starting on %s", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
