package data

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/media-service/internal/biz"
)

type mediaRepo struct {
	db *sql.DB
}

func NewMediaRepo(db *sql.DB) biz.MediaRepo {
	return &mediaRepo{db: db}
}

func (r *mediaRepo) CreateAttachment(ctx context.Context, attachment *biz.Attachment) error {
	metaJSON, _ := json.Marshal(attachment.Meta)

	query := `
		INSERT INTO attachments (id, message_id, object_key, file_name, mime_type, size, status, meta, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.db.ExecContext(ctx, query,
		attachment.ID, attachment.MessageID, attachment.ObjectKey, attachment.FileName,
		attachment.MimeType, attachment.Size, attachment.Status, metaJSON,
		attachment.CreatedAt, attachment.UpdatedAt)

	return err
}

func (r *mediaRepo) GetAttachment(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
	attachment := &biz.Attachment{}
	var metaJSON []byte

	query := `
		SELECT id, message_id, object_key, file_name, mime_type, size, status, meta, created_at, updated_at
		FROM attachments WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&attachment.ID, &attachment.MessageID, &attachment.ObjectKey, &attachment.FileName,
		&attachment.MimeType, &attachment.Size, &attachment.Status, &metaJSON,
		&attachment.CreatedAt, &attachment.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, biz.ErrAttachmentNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(metaJSON) > 0 {
		json.Unmarshal(metaJSON, &attachment.Meta)
	}

	return attachment, nil
}

func (r *mediaRepo) UpdateAttachment(ctx context.Context, attachment *biz.Attachment) error {
	metaJSON, _ := json.Marshal(attachment.Meta)

	query := `
		UPDATE attachments 
		SET message_id = $2, file_name = $3, mime_type = $4, size = $5, status = $6, meta = $7, updated_at = $8
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		attachment.ID, attachment.MessageID, attachment.FileName, attachment.MimeType,
		attachment.Size, attachment.Status, metaJSON, attachment.UpdatedAt)

	return err
}

func (r *mediaRepo) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM attachments WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *mediaRepo) GetAttachmentsByMessage(ctx context.Context, messageID uuid.UUID) ([]*biz.Attachment, error) {
	query := `
		SELECT id, message_id, object_key, file_name, mime_type, size, status, meta, created_at, updated_at
		FROM attachments 
		WHERE message_id = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []*biz.Attachment
	for rows.Next() {
		attachment := &biz.Attachment{}
		var metaJSON []byte

		err := rows.Scan(
			&attachment.ID, &attachment.MessageID, &attachment.ObjectKey, &attachment.FileName,
			&attachment.MimeType, &attachment.Size, &attachment.Status, &metaJSON,
			&attachment.CreatedAt, &attachment.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if len(metaJSON) > 0 {
			json.Unmarshal(metaJSON, &attachment.Meta)
		}
		attachments = append(attachments, attachment)
	}

	return attachments, nil
}
