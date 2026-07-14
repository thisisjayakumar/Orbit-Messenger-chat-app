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

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/media-service/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/media-service/internal/data"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/media-service/internal/server"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/config"
	sharedserver "github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/server"
)

func main() {
	// Database connection
	db, err := sql.Open("postgres", config.GetEnv("DATABASE_URL", "postgres://chat_user:chat_password@localhost:5432/chat_db?sslmode=disable"))
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Repository
	mediaRepo := data.NewMediaRepo(db)

	// MinIO Storage
	minioConfig := data.MinIOConfig{
		Endpoint:  config.GetEnv("MINIO_ENDPOINT", "localhost:9000"),
		AccessKey: config.GetEnv("MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey: config.GetEnv("MINIO_SECRET_KEY", "minioadmin123"),
		Bucket:    config.GetEnv("MINIO_BUCKET", "chat-attachments"),
		UseSSL:    false,
	}
	storage, err := data.NewMinIOStorage(minioConfig)
	if err != nil {
		log.Fatal("Failed to create MinIO storage:", err)
	}

	// Antivirus scanner (mock for now)
	antivirus := data.NewMockAntivirusScanner()

	// Use case
	mediaUc := biz.NewMediaUsecaseFromConfig(mediaRepo, storage, antivirus)

	// JWT secret for token validation (must match auth-service JWT_SECRET)
	jwtSecret := config.GetEnv("JWT_SECRET", "your-super-secret-jwt-key-change-in-production")

	// HTTP server
	httpServer := server.NewMediaHTTPServer(mediaUc, jwtSecret)

	// Start server
	srv := &http.Server{
		Addr:    ":" + config.GetEnv("PORT", "8004"),
		Handler: sharedserver.CORSMiddleware(httpServer),
	}

	go func() {
		log.Printf("Media service starting on port %s", config.GetEnv("PORT", "8004"))
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
