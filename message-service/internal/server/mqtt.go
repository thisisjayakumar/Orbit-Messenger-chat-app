package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/message-service/internal/biz"
)

type MQTTServer struct {
	client    mqtt.Client
	messageUc *biz.MessageUsecase
}

type MQTTConfig struct {
	BrokerURL string   `yaml:"broker_url"`
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	Topics    []string `yaml:"topics"`
}

func NewMQTTServer(config MQTTConfig, messageUc *biz.MessageUsecase) *MQTTServer {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.BrokerURL)
	opts.SetClientID("message-service")
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	server := &MQTTServer{
		messageUc: messageUc,
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
	if strings.Contains(topic, "/messages") {
		if err := s.messageUc.ProcessIncomingMessage(ctx, payload); err != nil {
			log.Printf("Error processing message: %v", err)
		}
	} else if strings.Contains(topic, "/typing") {
		if err := s.messageUc.ProcessTypingIndicator(ctx, payload); err != nil {
			log.Printf("Error processing typing indicator: %v", err)
		}
	}
}

func (s *MQTTServer) defaultMessageHandler(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message on unexpected topic %s: %s", msg.Topic(), string(msg.Payload()))
}
