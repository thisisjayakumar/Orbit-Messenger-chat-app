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

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/message-service/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/message-service/internal/data"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/message-service/internal/server"
)

func main() {
	// Database connection
	db, err := sql.Open("postgres", getEnv("DATABASE_URL", "postgres://chat_user:chat_password@localhost:5432/chat_db?sslmode=disable"))
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Repository
	messageRepo := data.NewMessageRepo(db)

	// Use case
	messageUc := biz.NewMessageUsecase(messageRepo)

	// MQTT server
	mqttConfig := server.MQTTConfig{
		BrokerURL: getEnv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		Username:  getEnv("MQTT_USERNAME", "message_service"),
		Password:  getEnv("MQTT_PASSWORD", "message_service_password"),
		Topics:    []string{"chat/+/messages", "chat/+/typing"},
	}
	mqttServer := server.NewMQTTServer(mqttConfig, messageUc)

	// Start MQTT server
	if err := mqttServer.Start(); err != nil {
		log.Fatal("Failed to start MQTT server:", err)
	}
	defer mqttServer.Stop()

	// Simple HTTP health check server
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start HTTP server for health checks
	srv := &http.Server{
		Addr:    ":" + getEnv("PORT", "8001"),
		Handler: nil,
	}

	go func() {
		log.Printf("Message service starting on port %s", getEnv("PORT", "8001"))
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
