package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/presence-service/internal/biz"
)

type MQTTServer struct {
	client      mqtt.Client
	presenceUc  *biz.PresenceUsecase
}

type MQTTConfig struct {
	BrokerURL string   `yaml:"broker_url"`
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	Topics    []string `yaml:"topics"`
}

func NewMQTTServer(config MQTTConfig, presenceUc *biz.PresenceUsecase) *MQTTServer {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.BrokerURL)
	opts.SetClientID("presence-service")
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	server := &MQTTServer{
		presenceUc: presenceUc,
	}

	opts.SetDefaultPublishHandler(server.defaultMessageHandler)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Println("Connected to MQTT broker")
		server.subscribeToTopics(config.Topics)
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("Connection lost: %v", err)
	})

	client := mqtt.NewClient(opts)
	server.client = client

	return server
}

func (s *MQTTServer) Start() error {
	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}
	return nil
}

func (s *MQTTServer) Stop() {
	s.client.Disconnect(250)
}

func (s *MQTTServer) subscribeToTopics(topics []string) {
	for _, topic := range topics {
		if token := s.client.Subscribe(topic, 1, s.messageHandler); token.Wait() && token.Error() != nil {
			log.Printf("Failed to subscribe to topic %s: %v", topic, token.Error())
		} else {
			log.Printf("Subscribed to topic: %s", topic)
		}
	}
}

func (s *MQTTServer) messageHandler(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	log.Printf("Received message on topic %s: %s", topic, string(payload))

	ctx := context.Background()

	// Route message based on topic pattern
	if strings.Contains(topic, "presence/") && strings.Contains(topic, "/status") {
		if err := s.presenceUc.HandlePresenceUpdate(ctx, payload); err != nil {
			log.Printf("Error processing presence update: %v", err)
		}
	} else if strings.Contains(topic, "connected") {
		s.handleClientConnected(ctx, topic, payload)
	} else if strings.Contains(topic, "disconnected") {
		s.handleClientDisconnected(ctx, topic, payload)
	}
}

func (s *MQTTServer) handleClientConnected(ctx context.Context, topic string, payload []byte) {
	// Extract client ID from topic: $SYS/brokers/+/clients/{clientID}/connected
	clientID := s.extractClientIDFromTopic(topic)
	if clientID == "" {
		return
	}

	// Parse connection info from payload
	var connInfo struct {
		ClientID   string `json:"clientid"`
		Username   string `json:"username"`
		IPAddress  string `json:"ipaddress"`
		ConnectedAt int64  `json:"connected_at"`
	}

	if err := json.Unmarshal(payload, &connInfo); err != nil {
		log.Printf("Error parsing connection info: %v", err)
		return
	}

	// Extract user ID from username (assuming username is user UUID)
	userID, err := uuid.Parse(connInfo.Username)
	if err != nil {
		log.Printf("Invalid user ID in username: %s", connInfo.Username)
		return
	}

	if err := s.presenceUc.HandleClientConnected(ctx, clientID, userID, "", connInfo.IPAddress); err != nil {
		log.Printf("Error handling client connected: %v", err)
	}
}

func (s *MQTTServer) handleClientDisconnected(ctx context.Context, topic string, payload []byte) {
	// Extract client ID from topic
	clientID := s.extractClientIDFromTopic(topic)
	if clientID == "" {
		return
	}

	if err := s.presenceUc.HandleClientDisconnected(ctx, clientID); err != nil {
		log.Printf("Error handling client disconnected: %v", err)
	}
}

func (s *MQTTServer) extractClientIDFromTopic(topic string) string {
	// Extract client ID from system topics like:
	// $SYS/brokers/+/clients/{clientID}/connected
	// $SYS/brokers/+/clients/{clientID}/disconnected
	re := regexp.MustCompile(`\$SYS/brokers/[^/]+/clients/([^/]+)/(connected|disconnected)`)
	matches := re.FindStringSubmatch(topic)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func (s *MQTTServer) defaultMessageHandler(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message on unexpected topic %s: %s", msg.Topic(), string(msg.Payload()))
}

// PublishPresenceUpdate publishes a presence update to MQTT
func (s *MQTTServer) PublishPresenceUpdate(userID uuid.UUID, status biz.PresenceStatus, customStatus string) error {
	topic := fmt.Sprintf("presence/%s/status", userID.String())
	
	update := biz.PresenceUpdate{
		UserID:       userID,
		Status:       status,
		CustomStatus: customStatus,
		Timestamp:    time.Now(),
	}

	payload, err := json.Marshal(update)
	if err != nil {
		return err
	}

	token := s.client.Publish(topic, 1, false, payload)
	token.Wait()
	return token.Error()
}

// StartCleanupRoutine starts a background routine to clean up stale presence data
func (s *MQTTServer) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Run cleanup every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.presenceUc.CleanupStalePresence(ctx); err != nil {
				log.Printf("Error during presence cleanup: %v", err)
			}
		}
	}
}
