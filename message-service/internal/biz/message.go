package biz

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID             uuid.UUID              `json:"id"`
	ConversationID uuid.UUID              `json:"conversation_id"`
	SenderID       uuid.UUID              `json:"sender_id"`
	ContentType    string                 `json:"content_type"`
	Content        string                 `json:"content"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
	DedupeKey      string                 `json:"dedupe_key,omitempty"`
	SentAt         time.Time              `json:"sent_at"`
	EditedAt       *time.Time             `json:"edited_at,omitempty"`
	Deleted        bool                   `json:"deleted"`
}

type IncomingMessage struct {
	ID             uuid.UUID              `json:"id"`
	ConversationID uuid.UUID              `json:"conversation_id"`
	SenderID       uuid.UUID              `json:"sender_id"`
	ContentType    string                 `json:"content_type"`
	Content        string                 `json:"content"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
	DedupeKey      string                 `json:"dedupe_key,omitempty"`
	SentAt         time.Time              `json:"sent_at"`
	Deleted        bool                   `json:"deleted"`
}

type Receipt struct {
	ID        uuid.UUID     `json:"id"`
	MessageID uuid.UUID     `json:"message_id"`
	UserID    uuid.UUID     `json:"user_id"`
	Status    ReceiptStatus `json:"status"`
	At        time.Time     `json:"at"`
}

type ReceiptStatus string

const (
	ReceiptStatusDelivered ReceiptStatus = "delivered"
	ReceiptStatusRead      ReceiptStatus = "read"
)

type Attachment struct {
	ID        uuid.UUID              `json:"id"`
	MessageID uuid.UUID              `json:"message_id"`
	ObjectKey string                 `json:"object_key"`
	MimeType  string                 `json:"mime_type"`
	Size      int64                  `json:"size"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

type TypingIndicator struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	UserID         uuid.UUID `json:"user_id"`
	IsTyping       bool      `json:"is_typing"`
	Timestamp      time.Time `json:"timestamp"`
}

type MessageRepo interface {
	GetMessage(ctx context.Context, id uuid.UUID) (*Message, error)
	GetMessagesByConversation(ctx context.Context, conversationID uuid.UUID, limit int, offset int) ([]*Message, error)
	UpdateMessage(ctx context.Context, message *Message) error
	DeleteMessage(ctx context.Context, id uuid.UUID) error

	CreateReceipt(ctx context.Context, receipt *Receipt) error
	GetReceiptsByMessage(ctx context.Context, messageID uuid.UUID) ([]*Receipt, error)

	CreateAttachment(ctx context.Context, attachment *Attachment) error
	GetAttachmentsByMessage(ctx context.Context, messageID uuid.UUID) ([]*Attachment, error)
}

type MessageUsecase struct {
	repo MessageRepo
}

func NewMessageUsecase(repo MessageRepo) *MessageUsecase {
	return &MessageUsecase{
		repo: repo,
	}
}

func (uc *MessageUsecase) ProcessIncomingMessage(ctx context.Context, payload []byte) error {
	var incoming IncomingMessage
	if err := json.Unmarshal(payload, &incoming); err != nil {
		return err
	}

	// Phase C: Message Service is consumer-only.
	// Message is already persisted by Chat API via transactional outbox.
	// Handle downstream tasks: receipts, push notifications, search indexing.
	log.Printf("[consumer] Received message %s in conversation %s from sender %s (type=%s)",
		incoming.ID, incoming.ConversationID, incoming.SenderID, incoming.ContentType)

	// TODO: Generate delivery receipts for online users
	// TODO: Trigger push notifications for offline users
	// TODO: Index message for search

	return nil
}

func (uc *MessageUsecase) ProcessTypingIndicator(ctx context.Context, payload []byte) error {
	var typing TypingIndicator
	if err := json.Unmarshal(payload, &typing); err != nil {
		return err
	}

	// For typing indicators, we typically just broadcast them
	// without persisting to database
	// This would be handled by the MQTT publisher
	return nil
}

func (uc *MessageUsecase) GetConversationMessages(ctx context.Context, conversationID uuid.UUID, limit int, offset int) ([]*Message, error) {
	return uc.repo.GetMessagesByConversation(ctx, conversationID, limit, offset)
}

func (uc *MessageUsecase) CreateReceipt(ctx context.Context, messageID, userID uuid.UUID, status ReceiptStatus) error {
	receipt := &Receipt{
		ID:        uuid.New(),
		MessageID: messageID,
		UserID:    userID,
		Status:    status,
		At:        time.Now(),
	}

	return uc.repo.CreateReceipt(ctx, receipt)
}

func (uc *MessageUsecase) EditMessage(ctx context.Context, messageID uuid.UUID, newContent string, senderID uuid.UUID) error {
	message, err := uc.repo.GetMessage(ctx, messageID)
	if err != nil {
		return err
	}

	// Verify sender owns the message
	if message.SenderID != senderID {
		return ErrUnauthorized
	}

	now := time.Now()
	message.Content = newContent
	message.EditedAt = &now

	return uc.repo.UpdateMessage(ctx, message)
}

func (uc *MessageUsecase) DeleteMessage(ctx context.Context, messageID uuid.UUID, senderID uuid.UUID) error {
	message, err := uc.repo.GetMessage(ctx, messageID)
	if err != nil {
		return err
	}

	// Verify sender owns the message
	if message.SenderID != senderID {
		return ErrUnauthorized
	}

	message.Deleted = true
	now := time.Now()
	message.EditedAt = &now

	return uc.repo.UpdateMessage(ctx, message)
}
