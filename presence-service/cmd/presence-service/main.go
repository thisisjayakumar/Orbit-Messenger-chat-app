package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/presence-service/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/presence-service/internal/data"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/presence-service/internal/server"
)

func main() {
	// Redis connection
	redisClient := redis.NewClient(&redis.Options{
		Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
		Password:     getEnv("REDIS_PASSWORD", ""),
		DB:           0,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	defer redisClient.Close()

	// Repository
	presenceRepo := data.NewPresenceRepo(redisClient)

	// Use case
	presenceUc := biz.NewPresenceUsecaseFromConfig(presenceRepo)

	// MQTT server
	mqttConfig := server.MQTTConfig{
		BrokerURL: getEnv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		Username:  getEnv("MQTT_USERNAME", "presence_service"),
		Password:  getEnv("MQTT_PASSWORD", "presence_service_password"),
		Topics:    []string{"presence/+/status", "$SYS/brokers/+/clients/+/connected", "$SYS/brokers/+/clients/+/disconnected"},
	}
	mqttServer := server.NewMQTTServer(mqttConfig, presenceUc)

	// Start MQTT server
	if err := mqttServer.Start(); err != nil {
		log.Fatal("Failed to start MQTT server:", err)
	}
	defer mqttServer.Stop()

	// HTTP server
	httpServer := server.NewPresenceHTTPServer(presenceUc, mqttServer)

	// Start server
	srv := &http.Server{
		Addr:    ":" + getEnv("PORT", "8002"),
		Handler: httpServer,
	}

	go func() {
		log.Printf("Presence service starting on port %s", getEnv("PORT", "8002"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Start cleanup routine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mqttServer.StartCleanupRoutine(ctx)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	cancel() // Stop cleanup routine
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
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