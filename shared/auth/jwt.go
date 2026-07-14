// Package auth provides shared JWT validation and typed context helpers
// for cross-service authentication. All services that require auth
// should use this package instead of header-based user identification.
package auth

import (
	"context"
	"errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrInvalidToken is returned when a JWT token is invalid or expired.
var ErrInvalidToken = errors.New("invalid token")

// ErrClaimsNotFound is returned when the context has no claims set.
var ErrClaimsNotFound = errors.New("auth claims not found in context")

// JWTClaims represents the claims in an Orbit Messenger JWT token.
type JWTClaims struct {
	UserID         int    `json:"user_id"`
	OrganizationID string `json:"organization_id"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	KeycloakID     string `json:"keycloak_id,omitempty"`
	jwt.RegisteredClaims
}

// typed context key to avoid collisions and prevent runtime panics
type contextKey struct{ name string }

var claimsKey = contextKey{"auth_claims"}

// ValidateToken parses and validates a JWT token string using the given secret.
// Returns the parsed claims on success, or ErrInvalidToken on failure.
func ValidateToken(tokenString, jwtSecret string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrInvalidToken
}

// SetClaims stores validated JWT claims in the context using a typed key.
// Returns a new context with the claims attached.
func SetClaims(ctx context.Context, claims *JWTClaims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// GetClaims retrieves JWT claims from the context.
// Returns ErrClaimsNotFound if no claims are present.
func GetClaims(ctx context.Context) (*JWTClaims, error) {
	claims, ok := ctx.Value(claimsKey).(*JWTClaims)
	if !ok || claims == nil {
		return nil, ErrClaimsNotFound
	}
	return claims, nil
}

// GetUserID extracts the user ID from JWT claims stored in the context.
// Returns 0 and ErrClaimsNotFound if no claims are present.
func GetUserID(ctx context.Context) (int, error) {
	claims, err := GetClaims(ctx)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}

// GetOrgID extracts the organization ID (as uuid.UUID) from JWT claims
// stored in the context. Returns uuid.Nil and an error if parsing fails
// or no claims are present.
func GetOrgID(ctx context.Context) (uuid.UUID, error) {
	claims, err := GetClaims(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	orgID, err := uuid.Parse(claims.OrganizationID)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return orgID, nil
}

// GetEmail extracts the email from JWT claims stored in the context.
// Returns empty string and ErrClaimsNotFound if no claims are present.
func GetEmail(ctx context.Context) (string, error) {
	claims, err := GetClaims(ctx)
	if err != nil {
		return "", err
	}
	return claims.Email, nil
}

// GetRole extracts the role string from JWT claims stored in the context.
// Returns empty string and ErrClaimsNotFound if no claims are present.
func GetRole(ctx context.Context) (string, error) {
	claims, err := GetClaims(ctx)
	if err != nil {
		return "", err
	}
	return claims.Role, nil
}
