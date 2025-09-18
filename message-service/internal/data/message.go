package data

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/message-service/internal/biz"
)

type messageRepo struct {
	db *sql.DB
}

func NewMessageRepo(db *sql.DB) biz.MessageRepo {
	return &messageRepo{db: db}
}

func (r *messageRepo) CreateMessage(ctx context.Context, message *biz.Message) error {
	metaJSON, _ := json.Marshal(message.Meta)

	query := `
		INSERT INTO messages (id, conversation_id, sender_id, content_type, content, meta, dedupe_key, sent_at, deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (conversation_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING`

	_, err := r.db.ExecContext(ctx, query,
		message.ID, message.ConversationID, message.SenderID, message.ContentType,
		message.Content, metaJSON, message.DedupeKey, message.SentAt, message.Deleted)

	return err
}

func (r *messageRepo) GetMessage(ctx context.Context, id uuid.UUID) (*biz.Message, error) {
	message := &biz.Message{}
	var metaJSON []byte

	query := `
		SELECT id, conversation_id, sender_id, content_type, content, meta, dedupe_key, sent_at, edited_at, deleted
		FROM messages WHERE id = $1 AND deleted = false`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&message.ID, &message.ConversationID, &message.SenderID, &message.ContentType,
		&message.Content, &metaJSON, &message.DedupeKey, &message.SentAt, &message.EditedAt, &message.Deleted)

	if err == sql.ErrNoRows {
		return nil, biz.ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(metaJSON, &message.Meta)
	return message, nil
}

func (r *messageRepo) GetMessagesByConversation(ctx context.Context, conversationID uuid.UUID, limit int, offset int) ([]*biz.Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, content_type, content, meta, dedupe_key, sent_at, edited_at, deleted
		FROM messages 
		WHERE conversation_id = $1 AND deleted = false
		ORDER BY sent_at DESC
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
			&message.Content, &metaJSON, &message.DedupeKey, &message.SentAt, &message.EditedAt, &message.Deleted)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(metaJSON, &message.Meta)
		messages = append(messages, message)
	}

	return messages, nil
}

func (r *messageRepo) UpdateMessage(ctx context.Context, message *biz.Message) error {
	metaJSON, _ := json.Marshal(message.Meta)

	query := `
		UPDATE messages 
		SET content = $2, meta = $3, edited_at = $4, deleted = $5
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		message.ID, message.Content, metaJSON, message.EditedAt, message.Deleted)

	return err
}

func (r *messageRepo) DeleteMessage(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE messages SET deleted = true, edited_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *messageRepo) CreateReceipt(ctx context.Context, receipt *biz.Receipt) error {
	query := `
		INSERT INTO message_receipts (id, message_id, user_id, status, at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (message_id, user_id, status) DO UPDATE SET at = $5`

	_, err := r.db.ExecContext(ctx, query,
		receipt.ID, receipt.MessageID, receipt.UserID, receipt.Status, receipt.At)

	return err
}

func (r *messageRepo) GetReceiptsByMessage(ctx context.Context, messageID uuid.UUID) ([]*biz.Receipt, error) {
	query := `
		SELECT id, message_id, user_id, status, at
		FROM message_receipts 
		WHERE message_id = $1
		ORDER BY at DESC`

	rows, err := r.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receipts []*biz.Receipt
	for rows.Next() {
		receipt := &biz.Receipt{}
		err := rows.Scan(&receipt.ID, &receipt.MessageID, &receipt.UserID, &receipt.Status, &receipt.At)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, receipt)
	}

	return receipts, nil
}

func (r *messageRepo) CreateAttachment(ctx context.Context, attachment *biz.Attachment) error {
	metaJSON, _ := json.Marshal(attachment.Meta)

	query := `
		INSERT INTO attachments (id, message_id, object_key, mime_type, size, meta)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.db.ExecContext(ctx, query,
		attachment.ID, attachment.MessageID, attachment.ObjectKey, 
		attachment.MimeType, attachment.Size, metaJSON)

	return err
}

func (r *messageRepo) GetAttachmentsByMessage(ctx context.Context, messageID uuid.UUID) ([]*biz.Attachment, error) {
	query := `
		SELECT id, message_id, object_key, mime_type, size, meta
		FROM attachments 
		WHERE message_id = $1`

	rows, err := r.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []*biz.Attachment
	for rows.Next() {
		attachment := &biz.Attachment{}
		var metaJSON []byte

		err := rows.Scan(&attachment.ID, &attachment.MessageID, &attachment.ObjectKey,
			&attachment.MimeType, &attachment.Size, &metaJSON)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(metaJSON, &attachment.Meta)
		attachments = append(attachments, attachment)
	}

	return attachments, nil
}
