package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/chat-api/internal/biz"
)

type mqttPublisher struct {
	client mqtt.Client
}

type MQTTConfig struct {
	BrokerURL string `yaml:"broker_url"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
}

func NewMQTTPublisher(config MQTTConfig) (biz.MQTTPublisher, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.BrokerURL)
	opts.SetClientID("chat-api-publisher")
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}

	return &mqttPublisher{client: client}, nil
}

func (p *mqttPublisher) PublishMessage(ctx context.Context, conversationID uuid.UUID, message *biz.Message) error {
	topic := fmt.Sprintf("chat/%s/messages", conversationID.String())
	
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	token := p.client.Publish(topic, 1, false, payload)
	token.Wait()
	return token.Error()
}

func (p *mqttPublisher) PublishTypingIndicator(ctx context.Context, conversationID, userID uuid.UUID, isTyping bool) error {
	topic := fmt.Sprintf("chat/%s/typing", conversationID.String())
	
	indicator := map[string]interface{}{
		"user_id":   userID.String(),
		"is_typing": isTyping,
		"timestamp": time.Now(),
	}

	payload, err := json.Marshal(indicator)
	if err != nil {
		return err
	}

	token := p.client.Publish(topic, 0, false, payload)
	token.Wait()
	return token.Error()
}
