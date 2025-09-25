package biz

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrUserExists      = errors.New("user already exists")
	ErrInvalidToken    = errors.New("invalid token")
)

type UserRole string

const (
	UserRoleAdmin  UserRole = "admin"
	UserRoleMember UserRole = "member"
)

type User struct {
	ID             int                    `json:"id"`
	OrganizationID uuid.UUID              `json:"organization_id"`
	Email          string                 `json:"email"`
	DisplayName    string                 `json:"display_name"`
	AvatarURL      string                 `json:"avatar_url,omitempty"`
	Role           UserRole               `json:"role"`
	Profile        map[string]interface{} `json:"profile,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	LastSeenAt     *time.Time             `json:"last_seen_at,omitempty"`
	PasswordHash   string                 `json:"-"`
	KeycloakID     string                 `json:"-"`
}

type Organization struct {
	ID        uuid.UUID              `json:"id"`
	Name      string                 `json:"name"`
	Settings  map[string]interface{} `json:"settings,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type RegisterRequest struct {
	Email            string     `json:"email" validate:"required,email"`
	Password         string     `json:"password" validate:"required,min=6"`
	DisplayName      string     `json:"display_name" validate:"required"`
	OrganizationID   *uuid.UUID `json:"organization_id,omitempty"`
	OrganizationName *string    `json:"organization_name,omitempty"`
}

type JWTClaims struct {
	UserID         int    `json:"user_id"`
	OrganizationID string `json:"organization_id"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	KeycloakID     string `json:"keycloak_id,omitempty"`
	jwt.RegisteredClaims
}

type OIDCLoginRequest struct {
	Code        string `json:"code" validate:"required"`
	RedirectURI string `json:"redirect_uri" validate:"required"`
	ClientID    string `json:"client_id" validate:"required"`
}

type KeycloakConfig struct {
	URL          string `yaml:"url"`
	Realm        string `yaml:"realm"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

type UpdateUserRequest struct {
	DisplayName *string                 `json:"display_name,omitempty"`
	AvatarURL   *string                 `json:"avatar_url,omitempty"`
	Role        *UserRole               `json:"role,omitempty"`
	Profile     *map[string]interface{} `json:"profile,omitempty"`
}

type AuthRepo interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string, orgID uuid.UUID) (*User, error)
	GetUserByEmailAnyOrg(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id int) (*User, error)
	GetUserByKeycloakID(ctx context.Context, keycloakID string) (*User, error)
	GetOrganizationUsers(ctx context.Context, orgID uuid.UUID) ([]*User, error)
	UpdateUser(ctx context.Context, userID int, req *UpdateUserRequest) error
	DeleteUser(ctx context.Context, userID int) error
	UpdateLastSeen(ctx context.Context, userID int) error

	CreateOrganization(ctx context.Context, org *Organization) error
	GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error)
}

type AuthUsecase struct {
	repo           AuthRepo
	jwtSecret      string
	tokenTTL       time.Duration
	keycloakConfig KeycloakConfig
	keycloakClient *gocloak.GoCloak
	oidcProvider   *oidc.Provider
}

func NewAuthUsecase(repo AuthRepo, jwtSecret string, tokenTTL time.Duration, keycloakConfig KeycloakConfig) (*AuthUsecase, error) {
	keycloakClient := gocloak.NewClient(keycloakConfig.URL)

	// Try to initialize OIDC provider, but don't fail if Keycloak is not available
	var oidcProvider *oidc.Provider
	oidcProvider, err := oidc.NewProvider(context.Background(), keycloakConfig.URL+"/realms/"+keycloakConfig.Realm)
	if err != nil {
		// Log warning but continue without Keycloak support
		log.Printf("Warning: Failed to initialize Keycloak OIDC provider: %v. Direct auth will still work.", err)
		oidcProvider = nil
	}

	return &AuthUsecase{
		repo:           repo,
		jwtSecret:      jwtSecret,
		tokenTTL:       tokenTTL,
		keycloakConfig: keycloakConfig,
		keycloakClient: keycloakClient,
		oidcProvider:   oidcProvider,
	}, nil
}

func (uc *AuthUsecase) Register(ctx context.Context, req *RegisterRequest) (*User, string, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}

	// Create or get organization
	var orgID uuid.UUID
	if req.OrganizationID != nil {
		orgID = *req.OrganizationID
		// Verify organization exists
		if _, err := uc.repo.GetOrganization(ctx, orgID); err != nil {
			return nil, "", err
		}
	} else if req.OrganizationName != nil {
		// Create new organization
		org := &Organization{
			ID:        uuid.New(),
			Name:      *req.OrganizationName,
			Settings:  make(map[string]interface{}),
			CreatedAt: time.Now(),
		}
		if err := uc.repo.CreateOrganization(ctx, org); err != nil {
			return nil, "", err
		}
		orgID = org.ID
	} else {
		return nil, "", errors.New("either organization_id or organization_name is required")
	}

	// Create user
	user := &User{
		OrganizationID: orgID,
		Email:          req.Email,
		DisplayName:    req.DisplayName,
		Role:           UserRoleMember, // Default role
		Profile:        make(map[string]interface{}),
		CreatedAt:      time.Now(),
		PasswordHash:   string(hashedPassword),
	}

	if err := uc.repo.CreateUser(ctx, user); err != nil {
		return nil, "", err
	}

	// Generate JWT token
	token, err := uc.generateToken(user)
	if err != nil {
		return nil, "", err
	}

	user.PasswordHash = "" // Don't return password hash
	return user, token, nil
}

func (uc *AuthUsecase) Login(ctx context.Context, req *LoginRequest, orgID uuid.UUID) (*User, string, error) {
	// Get user by email
	var user *User
	var err error

	// If no organization ID provided, find user in any organization
	if orgID == uuid.Nil {
		user, err = uc.repo.GetUserByEmailAnyOrg(ctx, req.Email)
	} else {
		user, err = uc.repo.GetUserByEmail(ctx, req.Email, orgID)
	}

	if err != nil {
		return nil, "", ErrUserNotFound
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, "", ErrInvalidPassword
	}

	// Update last seen
	uc.repo.UpdateLastSeen(ctx, user.ID)

	// Generate JWT token
	token, err := uc.generateToken(user)
	if err != nil {
		return nil, "", err
	}

	user.PasswordHash = "" // Don't return password hash
	return user, token, nil
}

func (uc *AuthUsecase) ValidateToken(ctx context.Context, tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(uc.jwtSecret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

func (uc *AuthUsecase) GetUser(ctx context.Context, userID int) (*User, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	user.PasswordHash = "" // Don't return password hash
	return user, nil
}

// OIDCLogin handles Keycloak OIDC authentication
func (uc *AuthUsecase) OIDCLogin(ctx context.Context, req *OIDCLoginRequest, orgID uuid.UUID) (*User, string, error) {
	if uc.oidcProvider == nil {
		return nil, "", errors.New("Keycloak OIDC provider not available")
	}

	// Exchange authorization code for token
	token, err := uc.keycloakClient.GetToken(ctx, uc.keycloakConfig.Realm, gocloak.TokenOptions{
		ClientID:     &uc.keycloakConfig.ClientID,
		ClientSecret: &uc.keycloakConfig.ClientSecret,
		Code:         &req.Code,
		RedirectURI:  &req.RedirectURI,
		GrantType:    gocloak.StringP("authorization_code"),
	})
	if err != nil {
		return nil, "", err
	}

	// Get user info from Keycloak
	userInfo, err := uc.keycloakClient.GetUserInfo(ctx, token.AccessToken, uc.keycloakConfig.Realm)
	if err != nil {
		return nil, "", err
	}

	// Check if user exists in our database
	user, err := uc.repo.GetUserByKeycloakID(ctx, *userInfo.Sub)
	if err != nil {
		// User doesn't exist, create new user
		user = &User{
			OrganizationID: orgID,
			Email:          *userInfo.Email,
			DisplayName:    *userInfo.Name,
			Role:           UserRoleMember,
			KeycloakID:     *userInfo.Sub,
			Profile:        make(map[string]interface{}),
			CreatedAt:      time.Now(),
		}

		if err := uc.repo.CreateUser(ctx, user); err != nil {
			return nil, "", err
		}
	}

	// Update last seen
	uc.repo.UpdateLastSeen(ctx, user.ID)

	// Generate JWT token
	jwtToken, err := uc.generateToken(user)
	if err != nil {
		return nil, "", err
	}

	return user, jwtToken, nil
}

// GenerateMQTTCredentials creates credentials for MQTT broker authentication
func (uc *AuthUsecase) GenerateMQTTCredentials(ctx context.Context, userID int) (string, string, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", err
	}

	// Generate MQTT username and password
	mqttUsername := fmt.Sprintf("user_%d", user.ID)
	mqttPassword, err := uc.generateToken(user)
	if err != nil {
		return "", "", err
	}

	return mqttUsername, mqttPassword, nil
}

// GetOrganizationUsers returns all users in the same organization
func (uc *AuthUsecase) GetOrganizationUsers(ctx context.Context, orgID uuid.UUID) ([]*User, error) {
	users, err := uc.repo.GetOrganizationUsers(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// Remove password hashes from response
	for _, user := range users {
		user.PasswordHash = ""
	}

	return users, nil
}

// UpdateUser updates user information (admin only)
func (uc *AuthUsecase) UpdateUser(ctx context.Context, requesterID, targetUserID int, req *UpdateUserRequest) error {
	// Get requester to check permissions
	requester, err := uc.repo.GetUserByID(ctx, requesterID)
	if err != nil {
		return err
	}

	// Only admins can update other users, users can update themselves (limited fields)
	if requesterID != targetUserID && requester.Role != UserRoleAdmin {
		return errors.New("insufficient permissions")
	}

	// If not admin, restrict what can be updated
	if requester.Role != UserRoleAdmin {
		// Non-admins can only update their own display name and avatar
		restrictedReq := &UpdateUserRequest{
			DisplayName: req.DisplayName,
			AvatarURL:   req.AvatarURL,
			Profile:     req.Profile,
		}
		return uc.repo.UpdateUser(ctx, targetUserID, restrictedReq)
	}

	return uc.repo.UpdateUser(ctx, targetUserID, req)
}

// DeleteUser deletes a user (admin only)
func (uc *AuthUsecase) DeleteUser(ctx context.Context, requesterID, targetUserID int) error {
	// Get requester to check permissions
	requester, err := uc.repo.GetUserByID(ctx, requesterID)
	if err != nil {
		return err
	}

	// Only admins can delete users
	if requester.Role != UserRoleAdmin {
		return errors.New("insufficient permissions")
	}

	// Cannot delete yourself
	if requesterID == targetUserID {
		return errors.New("cannot delete yourself")
	}

	return uc.repo.DeleteUser(ctx, targetUserID)
}

// IsAdmin checks if a user is an admin
func (uc *AuthUsecase) IsAdmin(ctx context.Context, userID int) (bool, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.Role == UserRoleAdmin, nil
}

func (uc *AuthUsecase) generateToken(user *User) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		UserID:         user.ID,
		OrganizationID: user.OrganizationID.String(),
		Email:          user.Email,
		Role:           string(user.Role),
		KeycloakID:     user.KeycloakID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(uc.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(uc.jwtSecret))
}
