package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/chat-api/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/chat-api/internal/data"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/chat-api/internal/server"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/config"
	sharedserver "github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/server"
)

func main() {
	log.Println("🚀 Starting Chat API (Phase C — outbox-only mode)")

	// Database connection
	db, err := sql.Open("postgres", config.GetEnv("DATABASE_URL", "postgres://chat_user:chat_password@localhost:5432/chat_db?sslmode=disable"))
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Repository
	chatRepo := data.NewChatRepo(db)

	// MQTT Publisher (typing indicators only — message delivery via Outbox Relay)
	mqttConfig := data.MQTTConfig{
		BrokerURL: config.GetEnv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		Username:  config.GetEnv("MQTT_USERNAME", "chat_api"),
		Password:  config.GetEnv("MQTT_PASSWORD", "chat_api_password"),
	}
	mqttPublisher, err := data.NewMQTTPublisher(mqttConfig)
	if err != nil {
		log.Fatal("Failed to create MQTT publisher:", err)
	}

	// Use case
	chatUc := biz.NewChatUsecase(chatRepo, mqttPublisher)

	// JWT secret for token validation (must match auth-service JWT_SECRET)
	jwtSecret := config.GetEnv("JWT_SECRET", "your-super-secret-jwt-key-change-in-production")

	// HTTP server
	httpServer := server.NewChatHTTPServer(chatUc, jwtSecret)

	// Start server
	srv := &http.Server{
		Addr:    ":" + config.GetEnv("PORT", "8003"),
		Handler: sharedserver.CORSMiddleware(httpServer),
	}

	go func() {
		log.Printf("Chat API starting on port %s", config.GetEnv("PORT", "8003"))
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
