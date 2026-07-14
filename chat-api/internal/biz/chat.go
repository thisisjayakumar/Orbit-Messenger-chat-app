package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ConversationType string

const (
	ConversationTypeDM    ConversationType = "DM"
	ConversationTypeGroup ConversationType = "GROUP"
)

type ParticipantRole string

const (
	ParticipantRoleAdmin  ParticipantRole = "admin"
	ParticipantRoleMember ParticipantRole = "member"
)

type Conversation struct {
	ID             uuid.UUID        `json:"id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	Type           ConversationType `json:"type"`
	Title          string           `json:"title,omitempty"`
	CreatedBy      int              `json:"created_by"`
	IsEncrypted    bool             `json:"is_encrypted"`
	CreatedAt      time.Time        `json:"created_at"`
}

type Participant struct {
	ID             uuid.UUID       `json:"id"`
	ConversationID uuid.UUID       `json:"conversation_id"`
	UserID         int             `json:"user_id"`
	Role           ParticipantRole `json:"role"`
	JoinedAt       time.Time       `json:"joined_at"`
	LastReadAt     *time.Time      `json:"last_read_at,omitempty"`
	DisplayName    string          `json:"display_name,omitempty"`
	Email          string          `json:"email,omitempty"`
}

type Message struct {
	ID             uuid.UUID              `json:"id"`
	ConversationID uuid.UUID              `json:"conversation_id"`
	SenderID       int                    `json:"sender_id"`
	ContentType    string                 `json:"content_type"`
	Content        string                 `json:"content"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
	DedupeKey      string                 `json:"dedupe_key,omitempty"`
	SentAt         time.Time              `json:"sent_at"`
	EditedAt       *time.Time             `json:"edited_at,omitempty"`
	Deleted        bool                   `json:"deleted"`
	IsRead         bool                   `json:"is_read"`
}

type CreateConversationRequest struct {
	Type           ConversationType `json:"type" validate:"required"`
	Title          string           `json:"title,omitempty"`
	ParticipantIDs []int            `json:"participant_ids" validate:"required"`
	IsEncrypted    bool             `json:"is_encrypted"`
}

type SendMessageRequest struct {
	ConversationID uuid.UUID              `json:"conversation_id" validate:"required"`
	ContentType    string                 `json:"content_type" validate:"required"`
	Content        string                 `json:"content" validate:"required"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
	DedupeKey      string                 `json:"dedupe_key,omitempty"`
}

type UpdateConversationRequest struct {
	Title *string `json:"title,omitempty"`
}

type AddParticipantRequest struct {
	UserID int             `json:"user_id" validate:"required"`
	Role   ParticipantRole `json:"role,omitempty"`
}

type ChatRepo interface {
	// Conversations
	CreateConversation(ctx context.Context, conversation *Conversation) error
	GetConversation(ctx context.Context, id uuid.UUID) (*Conversation, error)
	GetUserConversations(ctx context.Context, userID int) ([]*Conversation, error)
	UpdateConversation(ctx context.Context, conversation *Conversation) error
	DeleteConversation(ctx context.Context, id uuid.UUID) error

	// Participants
	AddParticipant(ctx context.Context, participant *Participant) error
	RemoveParticipant(ctx context.Context, conversationID uuid.UUID, userID int) error
	GetConversationParticipants(ctx context.Context, conversationID uuid.UUID) ([]*Participant, error)
	GetParticipant(ctx context.Context, conversationID uuid.UUID, userID int) (*Participant, error)
	UpdateParticipantRole(ctx context.Context, conversationID uuid.UUID, userID int, role ParticipantRole) error
	UpdateLastReadAt(ctx context.Context, conversationID uuid.UUID, userID int) error

	// Messages
	CreateMessageWithOutbox(ctx context.Context, message *Message, outbox *OutboxEntry) error
	GetConversationMessages(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*Message, error)
	GetMessage(ctx context.Context, messageID uuid.UUID) (*Message, error)
}

type OutboxEntry struct {
	MessageID      uuid.UUID       `json:"message_id"`
	ConversationID uuid.UUID       `json:"conversation_id"`
	Topic          string          `json:"topic"`
	Payload        json.RawMessage `json:"payload"`
}

type MQTTPublisher interface {
	PublishTypingIndicator(ctx context.Context, conversationID uuid.UUID, userID int, isTyping bool) error
}

type ChatUsecase struct {
	repo      ChatRepo
	publisher MQTTPublisher
}

func NewChatUsecase(repo ChatRepo, publisher MQTTPublisher) *ChatUsecase {
	return &ChatUsecase{
		repo:      repo,
		publisher: publisher,
	}
}

func (uc *ChatUsecase) CreateConversation(ctx context.Context, req *CreateConversationRequest, creatorID int, orgID uuid.UUID) (*Conversation, error) {
	// Validate participants
	if len(req.ParticipantIDs) == 0 {
		return nil, ErrInvalidRequest
	}

	// For DM conversations, ensure only 2 participants
	if req.Type == ConversationTypeDM && len(req.ParticipantIDs) != 1 {
		return nil, ErrInvalidDMParticipants
	}

	// Create conversation
	conversation := &Conversation{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Type:           req.Type,
		Title:          req.Title,
		CreatedBy:      creatorID,
		IsEncrypted:    req.IsEncrypted,
		CreatedAt:      time.Now(),
	}

	if err := uc.repo.CreateConversation(ctx, conversation); err != nil {
		return nil, err
	}

	// Add creator as admin participant
	creatorParticipant := &Participant{
		ID:             uuid.New(),
		ConversationID: conversation.ID,
		UserID:         creatorID,
		Role:           ParticipantRoleAdmin,
		JoinedAt:       time.Now(),
	}

	if err := uc.repo.AddParticipant(ctx, creatorParticipant); err != nil {
		return nil, err
	}

	// Add other participants
	for _, participantID := range req.ParticipantIDs {
		if participantID == creatorID {
			continue // Skip creator, already added
		}

		participant := &Participant{
			ID:             uuid.New(),
			ConversationID: conversation.ID,
			UserID:         participantID,
			Role:           ParticipantRoleMember,
			JoinedAt:       time.Now(),
		}

		if err := uc.repo.AddParticipant(ctx, participant); err != nil {
			return nil, err
		}
	}

	return conversation, nil
}

func (uc *ChatUsecase) GetUserConversations(ctx context.Context, userID int) ([]*Conversation, error) {
	return uc.repo.GetUserConversations(ctx, userID)
}

func (uc *ChatUsecase) GetConversation(ctx context.Context, conversationID uuid.UUID, userID int) (*Conversation, error) {
	// Check if user is participant
	participant, err := uc.repo.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}

	return uc.repo.GetConversation(ctx, conversationID)
}

func (uc *ChatUsecase) SendMessage(ctx context.Context, req *SendMessageRequest, senderID int) (*Message, error) {
	// Check if user is participant
	participant, err := uc.repo.GetParticipant(ctx, req.ConversationID, senderID)
	if err != nil {
		return nil, err
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}

	// Create message
	message := &Message{
		ID:             uuid.New(),
		ConversationID: req.ConversationID,
		SenderID:       senderID,
		ContentType:    req.ContentType,
		Content:        req.Content,
		Meta:           req.Meta,
		DedupeKey:      req.DedupeKey,
		SentAt:         time.Now(),
		Deleted:        false,
	}

	// Phase C: Write via transactional outbox (message + outbox in one PG transaction).
	// MQTT publish is handled by the Outbox Relay service.
	outbox := uc.buildOutboxEntry(message)
	if err := uc.repo.CreateMessageWithOutbox(ctx, message, outbox); err != nil {
		return nil, err
	}

	return message, nil
}

// buildOutboxEntry creates an outbox entry from a message for reliable MQTT delivery.
func (uc *ChatUsecase) buildOutboxEntry(message *Message) *OutboxEntry {
	payload, err := json.Marshal(message)
	if err != nil {
		// Marshalling a well-known struct should never fail
		payload = []byte{}
	}

	return &OutboxEntry{
		MessageID:      message.ID,
		ConversationID: message.ConversationID,
		Topic:          fmt.Sprintf("chat/%s/messages", message.ConversationID.String()),
		Payload:        payload,
	}
}

func (uc *ChatUsecase) GetConversationMessages(ctx context.Context, conversationID uuid.UUID, userID int, limit, offset int) ([]*Message, error) {
	// Check if user is participant
	participant, err := uc.repo.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}

	return uc.repo.GetConversationMessages(ctx, conversationID, limit, offset)
}

func (uc *ChatUsecase) AddParticipant(ctx context.Context, conversationID uuid.UUID, requesterID int, req *AddParticipantRequest) error {
	// Check if requester is admin
	requesterParticipant, err := uc.repo.GetParticipant(ctx, conversationID, requesterID)
	if err != nil {
		return err
	}
	if requesterParticipant == nil || requesterParticipant.Role != ParticipantRoleAdmin {
		return ErrInsufficientPermissions
	}

	// Add participant
	participant := &Participant{
		ID:             uuid.New(),
		ConversationID: conversationID,
		UserID:         req.UserID,
		Role:           req.Role,
		JoinedAt:       time.Now(),
	}

	if participant.Role == "" {
		participant.Role = ParticipantRoleMember
	}

	return uc.repo.AddParticipant(ctx, participant)
}

func (uc *ChatUsecase) RemoveParticipant(ctx context.Context, conversationID uuid.UUID, requesterID, targetUserID int) error {
	// Check if requester is admin or removing themselves
	requesterParticipant, err := uc.repo.GetParticipant(ctx, conversationID, requesterID)
	if err != nil {
		return err
	}
	if requesterParticipant == nil {
		return ErrNotParticipant
	}

	if requesterID != targetUserID && requesterParticipant.Role != ParticipantRoleAdmin {
		return ErrInsufficientPermissions
	}

	return uc.repo.RemoveParticipant(ctx, conversationID, targetUserID)
}

func (uc *ChatUsecase) UpdateConversation(ctx context.Context, conversationID uuid.UUID, requesterID int, req *UpdateConversationRequest) (*Conversation, error) {
	// Check if requester is admin
	requesterParticipant, err := uc.repo.GetParticipant(ctx, conversationID, requesterID)
	if err != nil {
		return nil, err
	}
	if requesterParticipant == nil || requesterParticipant.Role != ParticipantRoleAdmin {
		return nil, ErrInsufficientPermissions
	}

	conversation, err := uc.repo.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		conversation.Title = *req.Title
	}

	if err := uc.repo.UpdateConversation(ctx, conversation); err != nil {
		return nil, err
	}

	return conversation, nil
}

func (uc *ChatUsecase) MarkAsRead(ctx context.Context, conversationID uuid.UUID, userID int) error {
	// Check if user is participant
	participant, err := uc.repo.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if participant == nil {
		return ErrNotParticipant
	}

	return uc.repo.UpdateLastReadAt(ctx, conversationID, userID)
}

func (uc *ChatUsecase) SendTypingIndicator(ctx context.Context, conversationID uuid.UUID, userID int, isTyping bool) error {
	// Check if user is participant
	participant, err := uc.repo.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if participant == nil {
		return ErrNotParticipant
	}

	return uc.publisher.PublishTypingIndicator(ctx, conversationID, userID, isTyping)
}

func (uc *ChatUsecase) GetConversationParticipants(ctx context.Context, conversationID uuid.UUID, userID int) ([]*Participant, error) {
	// Check if user is participant
	participant, err := uc.repo.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}

	return uc.repo.GetConversationParticipants(ctx, conversationID)
}
