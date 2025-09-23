package data

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/chat-api/internal/biz"
)

type chatRepo struct {
	db *sql.DB
}

func NewChatRepo(db *sql.DB) biz.ChatRepo {
	return &chatRepo{db: db}
}

func (r *chatRepo) CreateConversation(ctx context.Context, conversation *biz.Conversation) error {
	query := `
		INSERT INTO conversations (id, organization_id, type, title, created_by, is_encrypted, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.ExecContext(ctx, query,
		conversation.ID, conversation.OrganizationID, conversation.Type, conversation.Title,
		conversation.CreatedBy, conversation.IsEncrypted, conversation.CreatedAt)

	return err
}

func (r *chatRepo) GetConversation(ctx context.Context, id uuid.UUID) (*biz.Conversation, error) {
	conversation := &biz.Conversation{}

	query := `
		SELECT id, organization_id, type, title, created_by, is_encrypted, created_at
		FROM conversations WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&conversation.ID, &conversation.OrganizationID, &conversation.Type, &conversation.Title,
		&conversation.CreatedBy, &conversation.IsEncrypted, &conversation.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, biz.ErrConversationNotFound
	}
	if err != nil {
		return nil, err
	}

	return conversation, nil
}

func (r *chatRepo) GetUserConversations(ctx context.Context, userID uuid.UUID) ([]*biz.Conversation, error) {
	query := `
		SELECT c.id, c.organization_id, c.type, c.title, c.created_by, c.is_encrypted, c.created_at
		FROM conversations c
		INNER JOIN conversation_participants cp ON c.id = cp.conversation_id
		WHERE cp.user_id = $1
		ORDER BY c.created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []*biz.Conversation
	for rows.Next() {
		conversation := &biz.Conversation{}
		err := rows.Scan(
			&conversation.ID, &conversation.OrganizationID, &conversation.Type, &conversation.Title,
			&conversation.CreatedBy, &conversation.IsEncrypted, &conversation.CreatedAt)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}

	return conversations, nil
}

func (r *chatRepo) UpdateConversation(ctx context.Context, conversation *biz.Conversation) error {
	query := `
		UPDATE conversations 
		SET title = $2, is_encrypted = $3
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, conversation.ID, conversation.Title, conversation.IsEncrypted)
	return err
}

func (r *chatRepo) DeleteConversation(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM conversations WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *chatRepo) AddParticipant(ctx context.Context, participant *biz.Participant) error {
	query := `
		INSERT INTO conversation_participants (id, conversation_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (conversation_id, user_id) DO NOTHING`

	_, err := r.db.ExecContext(ctx, query,
		participant.ID, participant.ConversationID, participant.UserID, participant.Role, participant.JoinedAt)

	return err
}

func (r *chatRepo) RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	query := `DELETE FROM conversation_participants WHERE conversation_id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, conversationID, userID)
	return err
}

func (r *chatRepo) GetConversationParticipants(ctx context.Context, conversationID uuid.UUID) ([]*biz.Participant, error) {
	query := `
		SELECT cp.id, cp.conversation_id, cp.user_id, cp.role, cp.joined_at, cp.last_read_at,
		       u.display_name, u.email
		FROM conversation_participants cp
		INNER JOIN users u ON cp.user_id = u.id
		WHERE cp.conversation_id = $1
		ORDER BY cp.joined_at ASC`

	rows, err := r.db.QueryContext(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []*biz.Participant
	for rows.Next() {
		participant := &biz.Participant{}
		err := rows.Scan(
			&participant.ID, &participant.ConversationID, &participant.UserID,
			&participant.Role, &participant.JoinedAt, &participant.LastReadAt,
			&participant.DisplayName, &participant.Email)
		if err != nil {
			return nil, err
		}
		participants = append(participants, participant)
	}

	return participants, nil
}

func (r *chatRepo) GetParticipant(ctx context.Context, conversationID, userID uuid.UUID) (*biz.Participant, error) {
	participant := &biz.Participant{}

	query := `
		SELECT id, conversation_id, user_id, role, joined_at, last_read_at
		FROM conversation_participants 
		WHERE conversation_id = $1 AND user_id = $2`

	err := r.db.QueryRowContext(ctx, query, conversationID, userID).Scan(
		&participant.ID, &participant.ConversationID, &participant.UserID,
		&participant.Role, &participant.JoinedAt, &participant.LastReadAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return participant, nil
}

func (r *chatRepo) UpdateParticipantRole(ctx context.Context, conversationID, userID uuid.UUID, role biz.ParticipantRole) error {
	query := `UPDATE conversation_participants SET role = $3 WHERE conversation_id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, conversationID, userID, role)
	return err
}

func (r *chatRepo) UpdateLastReadAt(ctx context.Context, conversationID, userID uuid.UUID) error {
	query := `UPDATE conversation_participants SET last_read_at = NOW() WHERE conversation_id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, conversationID, userID)
	return err
}

func (r *chatRepo) GetConversationMessages(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*biz.Message, error) {
	query := `
		SELECT m.id, m.conversation_id, m.sender_id, m.content_type, m.content, m.meta, m.dedupe_key, 
		       m.sent_at, m.edited_at, m.deleted,
		       CASE 
		           WHEN EXISTS (
		               SELECT 1 FROM conversation_participants cp 
		               WHERE cp.conversation_id = m.conversation_id 
		               AND cp.user_id != m.sender_id 
		               AND cp.last_read_at >= m.sent_at
		           ) THEN true 
		           ELSE false 
		       END as is_read
		FROM messages m
		WHERE m.conversation_id = $1 AND m.deleted = false
		ORDER BY m.sent_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*biz.Message
	for rows.Next() {
		message := &biz.Message{}
		var metaJSON []byte

		err := rows.Scan(
			&message.ID, &message.ConversationID, &message.SenderID, &message.ContentType,
			&message.Content, &metaJSON, &message.DedupeKey, &message.SentAt, &message.EditedAt, &message.Deleted, &message.IsRead)
		if err != nil {
			return nil, err
		}

		if len(metaJSON) > 0 {
			json.Unmarshal(metaJSON, &message.Meta)
		}
		messages = append(messages, message)
	}

	return messages, nil
}

func (r *chatRepo) GetMessage(ctx context.Context, messageID uuid.UUID) (*biz.Message, error) {
	message := &biz.Message{}
	var metaJSON []byte

	query := `
		SELECT id, conversation_id, sender_id, content_type, content, meta, dedupe_key, sent_at, edited_at, deleted
		FROM messages WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, messageID).Scan(
		&message.ID, &message.ConversationID, &message.SenderID, &message.ContentType,
		&message.Content, &metaJSON, &message.DedupeKey, &message.SentAt, &message.EditedAt, &message.Deleted)

	if err == sql.ErrNoRows {
		return nil, biz.ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(metaJSON) > 0 {
		json.Unmarshal(metaJSON, &message.Meta)
	}

	return message, nil
}
