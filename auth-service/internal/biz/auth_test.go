package biz

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ──────────────────────────────────────────────
// Mock AuthRepo
// ──────────────────────────────────────────────

type mockAuthRepo struct {
	createUserFunc            func(ctx context.Context, user *User) error
	getUserByEmailFunc        func(ctx context.Context, email string, orgID uuid.UUID) (*User, error)
	getUserByEmailAnyOrgFunc  func(ctx context.Context, email string) (*User, error)
	getUserByIDFunc           func(ctx context.Context, id int) (*User, error)
	getUserByUsernameFunc     func(ctx context.Context, username string, orgID uuid.UUID) (*User, error)
	searchUsersByUsernameFunc func(ctx context.Context, query string, orgID uuid.UUID, limit int) ([]*User, error)
	getOrganizationUsersFunc  func(ctx context.Context, orgID uuid.UUID) ([]*User, error)
	updateUserFunc            func(ctx context.Context, userID int, req *UpdateUserRequest) error
	deleteUserFunc            func(ctx context.Context, userID int) error
	updateLastSeenFunc        func(ctx context.Context, userID int) error
	createOrganizationFunc    func(ctx context.Context, org *Organization) error
	getOrganizationFunc       func(ctx context.Context, id uuid.UUID) (*Organization, error)
}

func (m *mockAuthRepo) CreateUser(ctx context.Context, user *User) error {
	if m.createUserFunc != nil {
		return m.createUserFunc(ctx, user)
	}
	return nil
}
func (m *mockAuthRepo) GetUserByEmail(ctx context.Context, email string, orgID uuid.UUID) (*User, error) {
	if m.getUserByEmailFunc != nil {
		return m.getUserByEmailFunc(ctx, email, orgID)
	}
	return nil, ErrUserNotFound
}
func (m *mockAuthRepo) GetUserByEmailAnyOrg(ctx context.Context, email string) (*User, error) {
	if m.getUserByEmailAnyOrgFunc != nil {
		return m.getUserByEmailAnyOrgFunc(ctx, email)
	}
	return nil, ErrUserNotFound
}
func (m *mockAuthRepo) GetUserByID(ctx context.Context, id int) (*User, error) {
	if m.getUserByIDFunc != nil {
		return m.getUserByIDFunc(ctx, id)
	}
	return nil, ErrUserNotFound
}
func (m *mockAuthRepo) GetUserByUsername(ctx context.Context, username string, orgID uuid.UUID) (*User, error) {
	if m.getUserByUsernameFunc != nil {
		return m.getUserByUsernameFunc(ctx, username, orgID)
	}
	return nil, ErrUserNotFound
}
func (m *mockAuthRepo) GetUserByKeycloakID(ctx context.Context, keycloakID string) (*User, error) {
	return nil, ErrUserNotFound
}
func (m *mockAuthRepo) SearchUsersByUsername(ctx context.Context, query string, orgID uuid.UUID, limit int) ([]*User, error) {
	if m.searchUsersByUsernameFunc != nil {
		return m.searchUsersByUsernameFunc(ctx, query, orgID, limit)
	}
	return nil, nil
}
func (m *mockAuthRepo) GetOrganizationUsers(ctx context.Context, orgID uuid.UUID) ([]*User, error) {
	if m.getOrganizationUsersFunc != nil {
		return m.getOrganizationUsersFunc(ctx, orgID)
	}
	return nil, nil
}
func (m *mockAuthRepo) UpdateUser(ctx context.Context, userID int, req *UpdateUserRequest) error {
	if m.updateUserFunc != nil {
		return m.updateUserFunc(ctx, userID, req)
	}
	return nil
}
func (m *mockAuthRepo) DeleteUser(ctx context.Context, userID int) error {
	if m.deleteUserFunc != nil {
		return m.deleteUserFunc(ctx, userID)
	}
	return nil
}
func (m *mockAuthRepo) UpdateLastSeen(ctx context.Context, userID int) error {
	if m.updateLastSeenFunc != nil {
		return m.updateLastSeenFunc(ctx, userID)
	}
	return nil
}
func (m *mockAuthRepo) CreateOrganization(ctx context.Context, org *Organization) error {
	if m.createOrganizationFunc != nil {
		return m.createOrganizationFunc(ctx, org)
	}
	return nil
}
func (m *mockAuthRepo) GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error) {
	if m.getOrganizationFunc != nil {
		return m.getOrganizationFunc(ctx, id)
	}
	return &Organization{ID: id, Name: "test-org", CreatedAt: time.Now()}, nil
}

// ──────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────

func newTestUsecase(repo *mockAuthRepo) *AuthUsecase {
	uc, err := NewAuthUsecase(repo, "test-secret", time.Hour, KeycloakConfig{})
	if err != nil {
		panic(err)
	}
	return uc
}

func validRegisterReq() *RegisterRequest {
	orgName := "acme-corp"
	return &RegisterRequest{
		Email:            "alice@example.com",
		Username:         "alice",
		Password:         "password123",
		DisplayName:      "Alice",
		OrganizationName: &orgName,
	}
}

func makeUser(id int, orgID uuid.UUID, role UserRole) *User {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	return &User{
		ID:             id,
		OrganizationID: orgID,
		Email:          "alice@example.com",
		Username:       "alice",
		DisplayName:    "Alice",
		Role:           role,
		Profile:        make(map[string]interface{}),
		CreatedAt:      time.Now(),
		PasswordHash:   string(hash),
	}
}

// ──────────────────────────────────────────────
// Tests: Register
// ──────────────────────────────────────────────

func TestRegister_Success_WithOrgName(t *testing.T) {
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	var createdOrgName string
	repo.createOrganizationFunc = func(ctx context.Context, org *Organization) error {
		createdOrgName = org.Name
		return nil
	}
	repo.createUserFunc = func(ctx context.Context, user *User) error {
		user.ID = 1 // Simulate assigned ID
		return nil
	}

	user, token, err := uc.Register(context.Background(), validRegisterReq())
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user == nil {
		t.Fatal("Register() returned nil user")
	}
	if token == "" {
		t.Fatal("Register() returned empty token")
	}
	if user.PasswordHash != "" {
		t.Error("password hash should be empty in returned user")
	}
	if user.ID != 1 {
		t.Errorf("user.ID = %d, want 1", user.ID)
	}
	if createdOrgName != "acme-corp" {
		t.Errorf("created org name = %q, want %q", createdOrgName, "acme-corp")
	}
	// Token should be parseable
	claims, err := uc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("generated token is invalid: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("claims.UserID = %d, want 1", claims.UserID)
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("claims.Email = %q, want %q", claims.Email, "alice@example.com")
	}
}

func TestRegister_Success_WithOrgID(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getOrganizationFunc = func(ctx context.Context, id uuid.UUID) (*Organization, error) {
		return &Organization{ID: orgID, Name: "existing-org"}, nil
	}
	repo.createUserFunc = func(ctx context.Context, user *User) error {
		user.ID = 1
		return nil
	}

	req := &RegisterRequest{
		Email:          "bob@example.com",
		Username:       "bob",
		Password:       "password123",
		DisplayName:    "Bob",
		OrganizationID: &orgID,
	}

	user, token, err := uc.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user.OrganizationID != orgID {
		t.Errorf("user.OrganizationID = %v, want %v", user.OrganizationID, orgID)
	}
	if token == "" {
		t.Error("token should not be empty")
	}
}

func TestRegister_MissingOrg_ReturnsError(t *testing.T) {
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	req := &RegisterRequest{
		Email:       "test@example.com",
		Username:    "testuser",
		Password:    "password123",
		DisplayName: "Test User",
		// Neither OrganizationID nor OrganizationName set
	}

	_, _, err := uc.Register(context.Background(), req)
	if err == nil {
		t.Fatal("Register() expected error for missing org, got nil")
	}
	if err.Error() != "either organization_id or organization_name is required" {
		t.Errorf("Register() error = %q, want 'either organization_id or organization_name is required'", err.Error())
	}
}

func TestRegister_UsernameAlreadyTaken_ReturnsError(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getOrganizationFunc = func(ctx context.Context, id uuid.UUID) (*Organization, error) {
		return &Organization{ID: orgID, Name: "existing-org"}, nil
	}
	repo.getUserByUsernameFunc = func(ctx context.Context, username string, oid uuid.UUID) (*User, error) {
		return &User{ID: 1, Username: username}, nil // Already exists
	}

	req := &RegisterRequest{
		Email:          "test@example.com",
		Username:       "alice",
		Password:       "password123",
		DisplayName:    "Test",
		OrganizationID: &orgID,
	}

	_, _, err := uc.Register(context.Background(), req)
	if err == nil {
		t.Fatal("Register() expected error for taken username, got nil")
	}
	if err.Error() != "username already taken" {
		t.Errorf("Register() error = %q, want 'username already taken'", err.Error())
	}
}

func TestRegister_InvalidUsername_ReturnsError(t *testing.T) {
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	req := &RegisterRequest{
		Email:            "test@example.com",
		Username:         "ab", // Too short (min 3 chars)
		Password:         "password123",
		DisplayName:      "Test",
		OrganizationName: strPtr("org"),
	}

	_, _, err := uc.Register(context.Background(), req)
	if err == nil {
		t.Fatal("Register() expected error for short username, got nil")
	}
	if err.Error() != "username must be between 3 and 30 characters" {
		t.Errorf("Register() error = %q, want username length error", err.Error())
	}

	// Test with reserved username
	req.Username = "admin"
	_, _, err = uc.Register(context.Background(), req)
	if err == nil {
		t.Fatal("Register() expected error for reserved username, got nil")
	}
	if err.Error() != "username is reserved" {
		t.Errorf("Register() error = %q, want 'username is reserved'", err.Error())
	}
}

func TestRegister_DuplicateEmail_ReturnsErrUserExists(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getOrganizationFunc = func(ctx context.Context, id uuid.UUID) (*Organization, error) {
		return &Organization{ID: orgID, Name: "existing-org"}, nil
	}
	repo.createUserFunc = func(ctx context.Context, user *User) error {
		return ErrUserExists
	}

	req := &RegisterRequest{
		Email:          "dup@example.com",
		Username:       "newuser",
		Password:       "password123",
		DisplayName:    "Dup",
		OrganizationID: &orgID,
	}

	_, _, err := uc.Register(context.Background(), req)
	if err != ErrUserExists {
		t.Errorf("Register() error = %v, want %v", err, ErrUserExists)
	}
}

// ──────────────────────────────────────────────
// Tests: Login
// ──────────────────────────────────────────────

func TestLogin_Success_WithOrgID(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByEmailFunc = func(ctx context.Context, email string, oid uuid.UUID) (*User, error) {
		user := makeUser(1, orgID, UserRoleMember)
		user.Email = email
		return user, nil
	}

	user, token, err := uc.Login(context.Background(), &LoginRequest{Email: "alice@example.com", Password: "password123"}, orgID)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if user == nil {
		t.Fatal("Login() returned nil user")
	}
	if token == "" {
		t.Fatal("Login() returned empty token")
	}
	if user.PasswordHash != "" {
		t.Error("password hash should be empty in returned user")
	}
}

func TestLogin_Success_AnyOrg(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByEmailAnyOrgFunc = func(ctx context.Context, email string) (*User, error) {
		return makeUser(1, orgID, UserRoleMember), nil
	}

	_, token, err := uc.Login(context.Background(), &LoginRequest{Email: "alice@example.com", Password: "password123"}, uuid.Nil)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if token == "" {
		t.Error("Login() returned empty token")
	}
}

func TestLogin_WrongPassword_ReturnsError(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByEmailFunc = func(ctx context.Context, email string, oid uuid.UUID) (*User, error) {
		return makeUser(1, orgID, UserRoleMember), nil
	}

	_, _, err := uc.Login(context.Background(), &LoginRequest{Email: "alice@example.com", Password: "wrongpassword"}, orgID)
	if err != ErrInvalidPassword {
		t.Errorf("Login() error = %v, want %v", err, ErrInvalidPassword)
	}
}

func TestLogin_UserNotFound_ReturnsError(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByEmailFunc = func(ctx context.Context, email string, oid uuid.UUID) (*User, error) {
		return nil, ErrUserNotFound
	}

	_, _, err := uc.Login(context.Background(), &LoginRequest{Email: "nobody@example.com", Password: "password123"}, orgID)
	if err != ErrUserNotFound {
		t.Errorf("Login() error = %v, want %v", err, ErrUserNotFound)
	}
}

// ──────────────────────────────────────────────
// Tests: ValidateToken
// ──────────────────────────────────────────────

func TestValidateToken_Valid(t *testing.T) {
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	orgID := uuid.New()
	user := makeUser(42, orgID, UserRoleAdmin)
	tokenStr, err := uc.generateToken(user)
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}

	claims, err := uc.ValidateToken(context.Background(), tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("claims.UserID = %d, want 42", claims.UserID)
	}
	if claims.OrganizationID != orgID.String() {
		t.Errorf("claims.OrganizationID = %s, want %s", claims.OrganizationID, orgID.String())
	}
	if claims.Role != "admin" {
		t.Errorf("claims.Role = %q, want %q", claims.Role, "admin")
	}
}

func TestValidateToken_WrongSecret_ReturnsError(t *testing.T) {
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	// Generate token with one secret...
	user := makeUser(1, uuid.New(), UserRoleMember)
	tokenStr, err := uc.generateToken(user)
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}

	// ...validate with a different secret
	uc2, _ := NewAuthUsecase(repo, "different-secret", time.Hour, KeycloakConfig{})
	_, err = uc2.ValidateToken(context.Background(), tokenStr)
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestValidateToken_Malformed_ReturnsError(t *testing.T) {
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	_, err := uc.ValidateToken(context.Background(), "not-a-jwt-token")
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestValidateToken_Expired_ReturnsError(t *testing.T) {
	repo := &mockAuthRepo{}
	// Use a negative TTL to create an already-expired token
	uc, _ := NewAuthUsecase(repo, "test-secret", -time.Hour, KeycloakConfig{})

	user := makeUser(1, uuid.New(), UserRoleMember)
	tokenStr, err := uc.generateToken(user)
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}

	_, err = uc.ValidateToken(context.Background(), tokenStr)
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

// ──────────────────────────────────────────────
// Tests: GetUser
// ──────────────────────────────────────────────

func TestGetUser_Found(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(id, orgID, UserRoleMember), nil
	}

	user, err := uc.GetUser(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if user.ID != 1 {
		t.Errorf("user.ID = %d, want 1", user.ID)
	}
	if user.PasswordHash != "" {
		t.Error("password hash should be empty")
	}
}

func TestGetUser_NotFound_ReturnsError(t *testing.T) {
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return nil, ErrUserNotFound
	}

	_, err := uc.GetUser(context.Background(), 999)
	if err != ErrUserNotFound {
		t.Errorf("GetUser() error = %v, want %v", err, ErrUserNotFound)
	}
}

// ──────────────────────────────────────────────
// Tests: validateUsername
// ──────────────────────────────────────────────

func TestValidateUsername_Valid(t *testing.T) {
	uc := newTestUsecase(&mockAuthRepo{})

	valid := []string{
		"john",
		"john_doe",
		"john.doe",
		"john123",
		"a.valid.username123",
		"test_user_42",
	}
	for _, name := range valid {
		if err := uc.validateUsername(name); err != nil {
			t.Errorf("validateUsername(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateUsername_TooShort(t *testing.T) {
	uc := newTestUsecase(&mockAuthRepo{})
	if err := uc.validateUsername("ab"); err == nil {
		t.Error("validateUsername('ab') expected error, got nil")
	}
}

func TestValidateUsername_TooLong(t *testing.T) {
	uc := newTestUsecase(&mockAuthRepo{})
	long := "aaaaaaaaaabbbbbbbbbbccccccccccdddddddddde" // 41 chars
	if err := uc.validateUsername(long); err == nil {
		t.Error("validateUsername(long) expected error, got nil")
	}
}

func TestValidateUsername_InvalidChars(t *testing.T) {
	uc := newTestUsecase(&mockAuthRepo{})

	invalid := []string{
		"user name",
		"user@name",
		"user-name",
		"user$name",
		"héllo",
	}
	for _, name := range invalid {
		if err := uc.validateUsername(name); err == nil {
			t.Errorf("validateUsername(%q) expected error, got nil", name)
		}
	}
}

func TestValidateUsername_StartsOrEndsWithPeriod(t *testing.T) {
	uc := newTestUsecase(&mockAuthRepo{})

	if err := uc.validateUsername(".john"); err == nil {
		t.Error("validateUsername('.john') expected error, got nil")
	}
	if err := uc.validateUsername("john."); err == nil {
		t.Error("validateUsername('john.') expected error, got nil")
	}
}

func TestValidateUsername_ConsecutivePeriods(t *testing.T) {
	uc := newTestUsecase(&mockAuthRepo{})
	if err := uc.validateUsername("john..doe"); err == nil {
		t.Error("validateUsername('john..doe') expected error, got nil")
	}
}

func TestValidateUsername_Reserved(t *testing.T) {
	uc := newTestUsecase(&mockAuthRepo{})

	reserved := []string{"admin", "root", "api", "www", "mail", "support", "help", "info", "about", "contact"}
	for _, name := range reserved {
		if err := uc.validateUsername(name); err == nil {
			t.Errorf("validateUsername(%q) expected error for reserved name, got nil", name)
		}
	}
}

// ──────────────────────────────────────────────
// Tests: UpdateUser
// ──────────────────────────────────────────────

func TestUpdateUser_AdminUpdatesOther_Success(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		if id == 1 {
			return makeUser(1, orgID, UserRoleAdmin), nil // requester is admin
		}
		return makeUser(2, orgID, UserRoleMember), nil
	}

	var updatedID int
	repo.updateUserFunc = func(ctx context.Context, userID int, req *UpdateUserRequest) error {
		updatedID = userID
		return nil
	}

	newDisplayName := "New Name"
	err := uc.UpdateUser(context.Background(), 1, 2, &UpdateUserRequest{DisplayName: &newDisplayName})
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if updatedID != 2 {
		t.Errorf("updatedID = %d, want 2", updatedID)
	}
}

func TestUpdateUser_NonAdminUpdatesOther_ReturnsError(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(id, orgID, UserRoleMember), nil // both are members
	}

	newName := "Hacker"
	err := uc.UpdateUser(context.Background(), 1, 2, &UpdateUserRequest{DisplayName: &newName})
	if err != ErrInsufficientPermissions {
		t.Errorf("UpdateUser() error = %v, want %v", err, ErrInsufficientPermissions)
	}
}

func TestUpdateUser_SelfUpdate_Success(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(id, orgID, UserRoleMember), nil
	}

	var capturedReq *UpdateUserRequest
	repo.updateUserFunc = func(ctx context.Context, userID int, req *UpdateUserRequest) error {
		capturedReq = req
		return nil
	}

	newName := "Self Update"
	err := uc.UpdateUser(context.Background(), 1, 1, &UpdateUserRequest{DisplayName: &newName})
	if err != nil {
		t.Fatalf("UpdateUser() self-update error = %v", err)
	}
	if capturedReq == nil || *capturedReq.DisplayName != "Self Update" {
		t.Error("self-update request should pass through display name")
	}
}

func TestUpdateUser_AdminUpdatesRole_Success(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	callCount := 0
	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		callCount++
		if callCount == 1 {
			return makeUser(1, orgID, UserRoleAdmin), nil // requester is admin
		}
		return makeUser(2, orgID, UserRoleMember), nil
	}

	var capturedReq *UpdateUserRequest
	repo.updateUserFunc = func(ctx context.Context, userID int, req *UpdateUserRequest) error {
		capturedReq = req
		return nil
	}

	newRole := UserRoleAdmin
	err := uc.UpdateUser(context.Background(), 1, 2, &UpdateUserRequest{Role: &newRole})
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if capturedReq == nil || *capturedReq.Role != UserRoleAdmin {
		t.Error("admin should be able to update user role")
	}
}

// ──────────────────────────────────────────────
// Tests: DeleteUser
// ──────────────────────────────────────────────

func TestDeleteUser_AdminDeletesOther_Success(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		if id == 1 {
			return makeUser(1, orgID, UserRoleAdmin), nil
		}
		return makeUser(2, orgID, UserRoleMember), nil
	}

	var deletedID int
	repo.deleteUserFunc = func(ctx context.Context, userID int) error {
		deletedID = userID
		return nil
	}

	err := uc.DeleteUser(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if deletedID != 2 {
		t.Errorf("deletedID = %d, want 2", deletedID)
	}
}

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(1, orgID, UserRoleAdmin), nil
	}

	err := uc.DeleteUser(context.Background(), 1, 1) // Same user
	if err != ErrCannotDeleteSelf {
		t.Errorf("DeleteUser() error = %v, want %v", err, ErrCannotDeleteSelf)
	}
}

func TestDeleteUser_NonAdmin_ReturnsError(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(id, orgID, UserRoleMember), nil
	}

	err := uc.DeleteUser(context.Background(), 1, 2)
	if err != ErrInsufficientPermissions {
		t.Errorf("DeleteUser() error = %v, want %v", err, ErrInsufficientPermissions)
	}
}

// ──────────────────────────────────────────────
// Tests: GetUserByUsername
// ──────────────────────────────────────────────

func TestGetUserByUsername_Found(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	callCount := 0
	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(1, orgID, UserRoleMember), nil
	}
	repo.getUserByUsernameFunc = func(ctx context.Context, username string, oid uuid.UUID) (*User, error) {
		callCount++
		return makeUser(42, orgID, UserRoleMember), nil
	}

	user, err := uc.GetUserByUsername(context.Background(), "alice", 1)
	if err != nil {
		t.Fatalf("GetUserByUsername() error = %v", err)
	}
	if user.ID != 42 {
		t.Errorf("user.ID = %d, want 42", user.ID)
	}
	if user.PasswordHash != "" {
		t.Error("password hash should be empty")
	}
}

func TestGetUserByUsername_NotFound_ReturnsError(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(1, orgID, UserRoleMember), nil
	}
	repo.getUserByUsernameFunc = func(ctx context.Context, username string, oid uuid.UUID) (*User, error) {
		return nil, ErrUserNotFound
	}

	_, err := uc.GetUserByUsername(context.Background(), "nonexistent", 1)
	if err != ErrUserNotFound {
		t.Errorf("GetUserByUsername() error = %v, want %v", err, ErrUserNotFound)
	}
}

// ──────────────────────────────────────────────
// Tests: SearchUsersByUsername
// ──────────────────────────────────────────────

func TestSearchUsersByUsername_ReturnsResults(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(1, orgID, UserRoleMember), nil
	}
	repo.searchUsersByUsernameFunc = func(ctx context.Context, query string, oid uuid.UUID, limit int) ([]*User, error) {
		return []*User{
			{ID: 2, Username: "alice", OrganizationID: orgID, PasswordHash: "secret"},
			{ID: 3, Username: "alex", OrganizationID: orgID, PasswordHash: "secret"},
		}, nil
	}

	users, err := uc.SearchUsersByUsername(context.Background(), "al", 1, 10)
	if err != nil {
		t.Fatalf("SearchUsersByUsername() error = %v", err)
	}
	if len(users) != 2 {
		t.Errorf("len(users) = %d, want 2", len(users))
	}
	for _, u := range users {
		if u.PasswordHash != "" {
			t.Errorf("password hash should be empty for user %d", u.ID)
		}
	}
}

func TestSearchUsersByUsername_EmptyResults(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(1, orgID, UserRoleMember), nil
	}
	repo.searchUsersByUsernameFunc = func(ctx context.Context, query string, oid uuid.UUID, limit int) ([]*User, error) {
		return []*User{}, nil
	}

	users, err := uc.SearchUsersByUsername(context.Background(), "zzz", 1, 10)
	if err != nil {
		t.Fatalf("SearchUsersByUsername() error = %v", err)
	}
	if len(users) != 0 {
		t.Errorf("len(users) = %d, want 0", len(users))
	}
}

// ──────────────────────────────────────────────
// Tests: GetOrganizationUsers
// ──────────────────────────────────────────────

func TestGetOrganizationUsers_ReturnsUsers(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getOrganizationUsersFunc = func(ctx context.Context, oid uuid.UUID) ([]*User, error) {
		return []*User{
			{ID: 1, Username: "alice", OrganizationID: oid, PasswordHash: "secret"},
			{ID: 2, Username: "bob", OrganizationID: oid, PasswordHash: "secret"},
		}, nil
	}

	users, err := uc.GetOrganizationUsers(context.Background(), orgID)
	if err != nil {
		t.Fatalf("GetOrganizationUsers() error = %v", err)
	}
	if len(users) != 2 {
		t.Errorf("len(users) = %d, want 2", len(users))
	}
	for _, u := range users {
		if u.PasswordHash != "" {
			t.Errorf("password hash should be empty for user %d", u.ID)
		}
	}
}

// ──────────────────────────────────────────────
// Tests: IsAdmin
// ──────────────────────────────────────────────

func TestIsAdmin_AdminUser_ReturnsTrue(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(1, orgID, UserRoleAdmin), nil
	}

	isAdmin, err := uc.IsAdmin(context.Background(), 1)
	if err != nil {
		t.Fatalf("IsAdmin() error = %v", err)
	}
	if !isAdmin {
		t.Error("IsAdmin() = false, want true")
	}
}

func TestIsAdmin_MemberUser_ReturnsFalse(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(1, orgID, UserRoleMember), nil
	}

	isAdmin, err := uc.IsAdmin(context.Background(), 1)
	if err != nil {
		t.Fatalf("IsAdmin() error = %v", err)
	}
	if isAdmin {
		t.Error("IsAdmin() = true, want false")
	}
}

func TestIsAdmin_UserNotFound_ReturnsError(t *testing.T) {
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return nil, ErrUserNotFound
	}

	_, err := uc.IsAdmin(context.Background(), 999)
	if err != ErrUserNotFound {
		t.Errorf("IsAdmin() error = %v, want %v", err, ErrUserNotFound)
	}
}

// ──────────────────────────────────────────────
// Tests: GenerateMQTTCredentials
// ──────────────────────────────────────────────

func TestGenerateMQTTCredentials_Success(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return makeUser(42, orgID, UserRoleMember), nil
	}

	mqttUser, mqttPass, err := uc.GenerateMQTTCredentials(context.Background(), 42)
	if err != nil {
		t.Fatalf("GenerateMQTTCredentials() error = %v", err)
	}
	if mqttUser != "user_42" {
		t.Errorf("mqttUser = %q, want %q", mqttUser, "user_42")
	}
	if mqttPass == "" {
		t.Error("mqttPass should not be empty")
	}
	// MQTT password should be a valid JWT
	claims, err := uc.ValidateToken(context.Background(), mqttPass)
	if err != nil {
		t.Errorf("mqtt password is not a valid JWT: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("claims.UserID = %d, want 42", claims.UserID)
	}
}

func TestGenerateMQTTCredentials_UserNotFound(t *testing.T) {
	repo := &mockAuthRepo{}
	uc := newTestUsecase(repo)

	repo.getUserByIDFunc = func(ctx context.Context, id int) (*User, error) {
		return nil, ErrUserNotFound
	}

	_, _, err := uc.GenerateMQTTCredentials(context.Background(), 999)
	if err != ErrUserNotFound {
		t.Errorf("GenerateMQTTCredentials() error = %v, want %v", err, ErrUserNotFound)
	}
}

// ──────────────────────────────────────────────
// Tests: NewAuthUsecase
// ──────────────────────────────────────────────

func TestNewAuthUsecase_DoesNotPanic(t *testing.T) {
	uc, err := NewAuthUsecase(&mockAuthRepo{}, "secret", time.Hour, KeycloakConfig{})
	if err != nil {
		t.Fatalf("NewAuthUsecase() error = %v", err)
	}
	if uc == nil {
		t.Fatal("NewAuthUsecase() returned nil")
	}
	if uc.jwtSecret != "secret" {
		t.Errorf("jwtSecret = %q, want %q", uc.jwtSecret, "secret")
	}
	if uc.tokenTTL != time.Hour {
		t.Errorf("tokenTTL = %v, want %v", uc.tokenTTL, time.Hour)
	}
}

// ──────────────────────────────────────────────
// Helper
// ──────────────────────────────────────────────

func strPtr(s string) *string {
	return &s
}
