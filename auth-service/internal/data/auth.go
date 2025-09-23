package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/lib/pq"
	_ "github.com/lib/pq"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/auth-service/internal/biz"
)

type authRepo struct {
	db *sql.DB
}

func NewAuthRepo(db *sql.DB) biz.AuthRepo {
	return &authRepo{db: db}
}

func (r *authRepo) CreateUser(ctx context.Context, user *biz.User) error {
	profileJSON, _ := json.Marshal(user.Profile)

	query := `
		INSERT INTO users (id, organization_id, email, display_name, avatar_url, profile, created_at, password_hash, keycloak_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, query,
		user.ID, user.OrganizationID, user.Email, user.DisplayName,
		user.AvatarURL, profileJSON, user.CreatedAt, user.PasswordHash, user.KeycloakID)

	if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
		return biz.ErrUserExists
	}

	return err
}

func (r *authRepo) GetUserByEmail(ctx context.Context, email string, orgID uuid.UUID) (*biz.User, error) {
	user := &biz.User{}
	var profileJSON []byte

	query := `
		SELECT id, organization_id, email, display_name, avatar_url, profile, created_at, last_seen_at, password_hash, keycloak_id
		FROM users WHERE email = $1 AND organization_id = $2`

	err := r.db.QueryRowContext(ctx, query, email, orgID).Scan(
		&user.ID, &user.OrganizationID, &user.Email, &user.DisplayName,
		&user.AvatarURL, &profileJSON, &user.CreatedAt, &user.LastSeenAt, &user.PasswordHash, &user.KeycloakID)

	if err == sql.ErrNoRows {
		return nil, biz.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(profileJSON, &user.Profile)
	return user, nil
}

func (r *authRepo) GetUserByEmailAnyOrg(ctx context.Context, email string) (*biz.User, error) {
	user := &biz.User{}
	var profileJSON []byte

	query := `
		SELECT id, organization_id, email, display_name, avatar_url, profile, created_at, last_seen_at, password_hash, keycloak_id
		FROM users WHERE email = $1 ORDER BY created_at DESC LIMIT 1`

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.OrganizationID, &user.Email, &user.DisplayName,
		&user.AvatarURL, &profileJSON, &user.CreatedAt, &user.LastSeenAt, &user.PasswordHash, &user.KeycloakID)

	if err == sql.ErrNoRows {
		return nil, biz.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(profileJSON, &user.Profile)
	return user, nil
}

func (r *authRepo) GetUserByID(ctx context.Context, id uuid.UUID) (*biz.User, error) {
	user := &biz.User{}
	var profileJSON []byte

	query := `
		SELECT id, organization_id, email, display_name, avatar_url, profile, created_at, last_seen_at, password_hash, keycloak_id
		FROM users WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.OrganizationID, &user.Email, &user.DisplayName,
		&user.AvatarURL, &profileJSON, &user.CreatedAt, &user.LastSeenAt, &user.PasswordHash, &user.KeycloakID)

	if err == sql.ErrNoRows {
		return nil, biz.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(profileJSON, &user.Profile)
	return user, nil
}

func (r *authRepo) GetUserByKeycloakID(ctx context.Context, keycloakID string) (*biz.User, error) {
	user := &biz.User{}
	var profileJSON []byte

	query := `
		SELECT id, organization_id, email, display_name, avatar_url, profile, created_at, last_seen_at, password_hash, keycloak_id
		FROM users WHERE keycloak_id = $1`

	err := r.db.QueryRowContext(ctx, query, keycloakID).Scan(
		&user.ID, &user.OrganizationID, &user.Email, &user.DisplayName,
		&user.AvatarURL, &profileJSON, &user.CreatedAt, &user.LastSeenAt, &user.PasswordHash, &user.KeycloakID)

	if err == sql.ErrNoRows {
		return nil, biz.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(profileJSON, &user.Profile)
	return user, nil
}

func (r *authRepo) UpdateLastSeen(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE users SET last_seen_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

func (r *authRepo) CreateOrganization(ctx context.Context, org *biz.Organization) error {
	settingsJSON, _ := json.Marshal(org.Settings)

	query := `
		INSERT INTO organizations (id, name, settings, created_at)
		VALUES ($1, $2, $3, $4)`

	_, err := r.db.ExecContext(ctx, query, org.ID, org.Name, settingsJSON, org.CreatedAt)
	return err
}

func (r *authRepo) GetOrganization(ctx context.Context, id uuid.UUID) (*biz.Organization, error) {
	org := &biz.Organization{}
	var settingsJSON []byte

	query := `SELECT id, name, settings, created_at FROM organizations WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&org.ID, &org.Name, &settingsJSON, &org.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, errors.New("organization not found")
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(settingsJSON, &org.Settings)
	return org, nil
}
