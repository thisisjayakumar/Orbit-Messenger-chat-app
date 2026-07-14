package biz

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Mock implementations
// ──────────────────────────────────────────────

type mockChatRepo struct {
	// Hook functions — set these to control mock behavior
	getParticipantFunc      func(ctx context.Context, conversationID uuid.UUID, userID int) (*Participant, error)
	createMessageOutboxFunc func(ctx context.Context, message *Message, outbox *OutboxEntry) error
}

func (m *mockChatRepo) CreateConversation(ctx context.Context, conversation *Conversation) error {
	return nil
}
func (m *mockChatRepo) GetConversation(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	return nil, nil
}
func (m *mockChatRepo) GetUserConversations(ctx context.Context, userID int) ([]*Conversation, error) {
	return nil, nil
}
func (m *mockChatRepo) UpdateConversation(ctx context.Context, conversation *Conversation) error {
	return nil
}
func (m *mockChatRepo) DeleteConversation(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *mockChatRepo) AddParticipant(ctx context.Context, participant *Participant) error {
	return nil
}
func (m *mockChatRepo) RemoveParticipant(ctx context.Context, conversationID uuid.UUID, userID int) error {
	return nil
}
func (m *mockChatRepo) GetConversationParticipants(ctx context.Context, conversationID uuid.UUID) ([]*Participant, error) {
	return nil, nil
}
func (m *mockChatRepo) GetParticipant(ctx context.Context, conversationID uuid.UUID, userID int) (*Participant, error) {
	if m.getParticipantFunc != nil {
		return m.getParticipantFunc(ctx, conversationID, userID)
	}
	return &Participant{UserID: userID, Role: ParticipantRoleMember}, nil
}
func (m *mockChatRepo) UpdateParticipantRole(ctx context.Context, conversationID uuid.UUID, userID int, role ParticipantRole) error {
	return nil
}
func (m *mockChatRepo) UpdateLastReadAt(ctx context.Context, conversationID uuid.UUID, userID int) error {
	return nil
}
func (m *mockChatRepo) CreateMessageWithOutbox(ctx context.Context, message *Message, outbox *OutboxEntry) error {
	if m.createMessageOutboxFunc != nil {
		return m.createMessageOutboxFunc(ctx, message, outbox)
	}
	return nil
}
func (m *mockChatRepo) GetConversationMessages(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*Message, error) {
	return nil, nil
}
func (m *mockChatRepo) GetMessage(ctx context.Context, messageID uuid.UUID) (*Message, error) {
	return nil, nil
}

type mockMQTTPublisher struct {
	publishTypingIndicatorFunc func(ctx context.Context, conversationID uuid.UUID, userID int, isTyping bool) error
}

func (m *mockMQTTPublisher) PublishTypingIndicator(ctx context.Context, conversationID uuid.UUID, userID int, isTyping bool) error {
	if m.publishTypingIndicatorFunc != nil {
		return m.publishTypingIndicatorFunc(ctx, conversationID, userID, isTyping)
	}
	return nil
}

// ──────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────

func validRequest() *SendMessageRequest {
	return &SendMessageRequest{
		ConversationID: uuid.New(),
		ContentType:    "text/plain",
		Content:        "Hello, world!",
		DedupeKey:      "client-key-123",
	}
}

func defaultRepo() *mockChatRepo {
	return &mockChatRepo{}
}

func defaultPublisher() *mockMQTTPublisher {
	return &mockMQTTPublisher{}
}

// ──────────────────────────────────────────────
// Tests: SendMessage — Phase C (outbox-only)
// ──────────────────────────────────────────────

func TestSendMessage_CallsCreateMessageWithOutbox(t *testing.T) {
	repo := defaultRepo()
	publisher := defaultPublisher()
	uc := NewChatUsecase(repo, publisher)

	var outboxCalled bool
	repo.createMessageOutboxFunc = func(ctx context.Context, msg *Message, out *OutboxEntry) error {
		outboxCalled = true
		if out.MessageID != msg.ID {
			t.Errorf("outbox.MessageID = %v, want %v", out.MessageID, msg.ID)
		}
		if out.ConversationID != msg.ConversationID {
			t.Errorf("outbox.ConversationID = %v, want %v", out.ConversationID, msg.ConversationID)
		}
		if out.Topic != "chat/"+msg.ConversationID.String()+"/messages" {
			t.Errorf("outbox.Topic = %q, want chat/%s/messages", out.Topic, msg.ConversationID.String())
		}
		if len(out.Payload) == 0 {
			t.Error("outbox.Payload is empty")
		}
		return nil
	}

	msg, err := uc.SendMessage(context.Background(), validRequest(), 1)
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if msg == nil {
		t.Fatal("SendMessage() returned nil message")
	}
	if !outboxCalled {
		t.Error("CreateMessageWithOutbox was not called")
	}
}

func TestSendMessage_RepoError_Propagates(t *testing.T) {
	repo := defaultRepo()
	publisher := defaultPublisher()
	uc := NewChatUsecase(repo, publisher)

	expectedErr := errors.New("db connection lost")
	repo.createMessageOutboxFunc = func(ctx context.Context, msg *Message, out *OutboxEntry) error {
		return expectedErr
	}

	_, err := uc.SendMessage(context.Background(), validRequest(), 1)
	if err == nil {
		t.Fatal("SendMessage() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("SendMessage() error = %v, want %v", err, expectedErr)
	}
}

// ──────────────────────────────────────────────
// Tests: SendMessage — participant validation
// ──────────────────────────────────────────────

func TestSendMessage_NotParticipant_ReturnsError(t *testing.T) {
	repo := defaultRepo()
	publisher := defaultPublisher()
	uc := NewChatUsecase(repo, publisher)

	repo.getParticipantFunc = func(ctx context.Context, conversationID uuid.UUID, userID int) (*Participant, error) {
		return nil, nil // nil participant = not part of conversation
	}

	_, err := uc.SendMessage(context.Background(), validRequest(), 1)
	if err != ErrNotParticipant {
		t.Errorf("SendMessage() error = %v, want %v", err, ErrNotParticipant)
	}
}

func TestSendMessage_GetParticipantDBError_ReturnsError(t *testing.T) {
	repo := defaultRepo()
	publisher := defaultPublisher()
	uc := NewChatUsecase(repo, publisher)

	expectedErr := errors.New("db error")
	repo.getParticipantFunc = func(ctx context.Context, conversationID uuid.UUID, userID int) (*Participant, error) {
		return nil, expectedErr
	}

	_, err := uc.SendMessage(context.Background(), validRequest(), 1)
	if err != expectedErr {
		t.Errorf("SendMessage() error = %v, want %v", err, expectedErr)
	}
}

func TestSendMessage_WithDedupeKey_PassesKeyToMessage(t *testing.T) {
	repo := defaultRepo()
	publisher := defaultPublisher()
	uc := NewChatUsecase(repo, publisher)

	var capturedMsg *Message
	repo.createMessageOutboxFunc = func(ctx context.Context, msg *Message, out *OutboxEntry) error {
		capturedMsg = msg
		return nil
	}

	req := validRequest()
	req.DedupeKey = "my-unique-key-456"

	msg, err := uc.SendMessage(context.Background(), req, 1)
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if capturedMsg == nil || msg == nil {
		t.Fatal("message was nil")
	}
	if capturedMsg.DedupeKey != "my-unique-key-456" {
		t.Errorf("DedupeKey = %q, want %q", capturedMsg.DedupeKey, "my-unique-key-456")
	}
}

func TestSendMessage_ContentTypeAndContent_Preserved(t *testing.T) {
	repo := defaultRepo()
	publisher := defaultPublisher()
	uc := NewChatUsecase(repo, publisher)

	var capturedMsg *Message
	repo.createMessageOutboxFunc = func(ctx context.Context, msg *Message, out *OutboxEntry) error {
		capturedMsg = msg
		return nil
	}

	req := &SendMessageRequest{
		ConversationID: uuid.New(),
		ContentType:    "application/json",
		Content:        `{"text":"hello"}`,
		Meta: map[string]interface{}{
			"edited": true,
			"font":   "bold",
		},
	}

	msg, err := uc.SendMessage(context.Background(), req, 42)
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if capturedMsg.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want %q", capturedMsg.ContentType, "application/json")
	}
	if capturedMsg.Content != `{"text":"hello"}` {
		t.Errorf("Content = %q, want %q", capturedMsg.Content, `{"text":"hello"}`)
	}
	if capturedMsg.SenderID != 42 {
		t.Errorf("SenderID = %d, want %d", capturedMsg.SenderID, 42)
	}
	if capturedMsg.Deleted != false {
		t.Error("Deleted should be false for new messages")
	}
	if capturedMsg.EditedAt != nil {
		t.Error("EditedAt should be nil for new messages")
	}
	if capturedMsg.SentAt.IsZero() {
		t.Error("SentAt should be set (non-zero)")
	}
	if capturedMsg.ID == uuid.Nil {
		t.Error("ID should be set (non-nil)")
	}
	// Verify Meta is preserved
	if meta, ok := capturedMsg.Meta["edited"]; !ok || meta != true {
		t.Error("Meta['edited'] should be true")
	}
	if meta, ok := capturedMsg.Meta["font"]; !ok || meta != "bold" {
		t.Error("Meta['font'] should be 'bold'")
	}
	// Verify the message returned to caller matches
	if msg.ID != capturedMsg.ID {
		t.Error("returned message ID doesn't match captured")
	}
}

// ──────────────────────────────────────────────
// Tests: buildOutboxEntry
// ──────────────────────────────────────────────

func TestBuildOutboxEntry_TopicFormat(t *testing.T) {
	uc := NewChatUsecase(defaultRepo(), defaultPublisher())

	convID := uuid.New()
	msg := &Message{
		ID:             uuid.New(),
		ConversationID: convID,
		SenderID:       1,
		ContentType:    "text/plain",
		Content:        "test",
		SentAt:         time.Now(),
	}

	entry := uc.buildOutboxEntry(msg)
	expectedTopic := "chat/" + convID.String() + "/messages"
	if entry.Topic != expectedTopic {
		t.Errorf("Topic = %q, want %q", entry.Topic, expectedTopic)
	}
}

func TestBuildOutboxEntry_PayloadContainsMessageData(t *testing.T) {
	uc := NewChatUsecase(defaultRepo(), defaultPublisher())

	msg := &Message{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		SenderID:       7,
		ContentType:    "text/plain",
		Content:        "Hello from test",
		DedupeKey:      "dk-999",
		SentAt:         time.Now(),
	}

	entry := uc.buildOutboxEntry(msg)

	// Parse the payload back and verify fields
	var decoded Message
	if err := json.Unmarshal(entry.Payload, &decoded); err != nil {
		t.Fatalf("failed to unmarshal outbox payload: %v", err)
	}

	if decoded.ID != msg.ID {
		t.Errorf("payload.ID = %v, want %v", decoded.ID, msg.ID)
	}
	if decoded.ConversationID != msg.ConversationID {
		t.Errorf("payload.ConversationID = %v, want %v", decoded.ConversationID, msg.ConversationID)
	}
	if decoded.SenderID != msg.SenderID {
		t.Errorf("payload.SenderID = %d, want %d", decoded.SenderID, msg.SenderID)
	}
	if decoded.Content != msg.Content {
		t.Errorf("payload.Content = %q, want %q", decoded.Content, msg.Content)
	}
	if decoded.DedupeKey != msg.DedupeKey {
		t.Errorf("payload.DedupeKey = %q, want %q", decoded.DedupeKey, msg.DedupeKey)
	}
}

func TestBuildOutboxEntry_MessageIDMatchesEntryID(t *testing.T) {
	uc := NewChatUsecase(defaultRepo(), defaultPublisher())

	msgID := uuid.New()
	msg := &Message{
		ID:             msgID,
		ConversationID: uuid.New(),
		SenderID:       1,
		ContentType:    "text/plain",
		Content:        "test",
	}

	entry := uc.buildOutboxEntry(msg)
	if entry.MessageID != msgID {
		t.Errorf("entry.MessageID = %v, want %v", entry.MessageID, msgID)
	}
}

func TestBuildOutboxEntry_ConversationIDMatches(t *testing.T) {
	uc := NewChatUsecase(defaultRepo(), defaultPublisher())

	convID := uuid.New()
	msg := &Message{
		ID:             uuid.New(),
		ConversationID: convID,
		SenderID:       1,
		ContentType:    "text/plain",
		Content:        "test",
	}

	entry := uc.buildOutboxEntry(msg)
	if entry.ConversationID != convID {
		t.Errorf("entry.ConversationID = %v, want %v", entry.ConversationID, convID)
	}
}

// ──────────────────────────────────────────────
// Tests: NewChatUsecase
// ──────────────────────────────────────────────

func TestNewChatUsecase_CreateMessageWithOutboxCalled(t *testing.T) {
	repo := defaultRepo()
	publisher := defaultPublisher()
	uc := NewChatUsecase(repo, publisher)

	var called bool
	repo.createMessageOutboxFunc = func(ctx context.Context, msg *Message, out *OutboxEntry) error {
		called = true
		return nil
	}

	uc.SendMessage(context.Background(), validRequest(), 1)
	if !called {
		t.Error("CreateMessageWithOutbox was not called")
	}
}
