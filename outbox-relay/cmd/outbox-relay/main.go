package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/lib/pq"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/outbox-relay/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/outbox-relay/internal/data"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/config"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("🚀 Starting Outbox Relay Service...")

	// ──────────── Database ────────────
	db, err := sql.Open("postgres", config.GetEnv("DATABASE_URL", "postgres://chat_user:chat_password@localhost:5432/chat_db?sslmode=disable"))
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("✅ Connected to PostgreSQL")

	// ──────────── Outbox Repo ────────────
	repo := data.NewOutboxRepo(db)

	// ──────────── MQTT Publisher ────────────
	mqttClient := newMQTTClient()
	mqttPublisher := &mqttRelayPublisher{client: mqttClient}

	// ──────────── Relay Config ────────────
	relayConfig := biz.RelayConfig{
		PollInterval: config.GetDuration("POLL_INTERVAL", 500*time.Millisecond),
		BatchSize:    config.GetInt("BATCH_SIZE", 100),
		MaxRetries:   config.GetInt("MAX_RETRIES", 5),
		RetryBackoff: config.GetDuration("RETRY_BACKOFF", 2*time.Second),
		MaxBackoff:   24 * time.Hour,
		PendingAge:   config.GetDuration("PENDING_AGE", 1*time.Second),
	}

	log.Printf("📋 Relay config: poll_interval=%v batch=%d max_retries=%d backoff=%v",
		relayConfig.PollInterval, relayConfig.BatchSize, relayConfig.MaxRetries, relayConfig.RetryBackoff)

	// ──────────── Relay Service ────────────
	relay := biz.NewRelayService(repo, mqttPublisher, relayConfig)

	// ──────────── HTTP Health Server ────────────
	httpAddr := ":" + config.GetEnv("PORT", "8005")
	httpServer := &http.Server{
		Addr: httpAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok","service":"outbox-relay"}`))
				return
			}
			http.NotFound(w, r)
		}),
	}

	// ──────────── Graceful Shutdown ────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Start HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("📡 Health check server on %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Start relay
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := relay.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("Relay service error: %v", err)
		}
	}()

	// ──────────── Wait for signal ────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("⚠️  Received signal: %v — shutting down...", sig)

	// Graceful shutdown
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server forced shutdown: %v", err)
	}

	wg.Wait()
	log.Println("✅ Outbox Relay Service stopped")
}

// ──────────── MQTT Client ────────────

func newMQTTClient() mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.GetEnv("MQTT_BROKER_URL", "tcp://localhost:1883"))
	opts.SetClientID("outbox-relay")
	opts.SetUsername(config.GetEnv("MQTT_USERNAME", "outbox_relay"))
	opts.SetPassword(config.GetEnv("MQTT_PASSWORD", "outbox_relay_password"))
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", token.Error())
	}
	log.Println("✅ Connected to MQTT broker")
	return client
}

// mqttRelayPublisher adapts an mqtt.Client to the biz.MQTTPublisher interface.
type mqttRelayPublisher struct {
	client mqtt.Client
}

func (p *mqttRelayPublisher) Publish(topic string, qos byte, payload []byte) error {
	token := p.client.Publish(topic, qos, false, payload)
	token.Wait()
	return token.Error()
}
