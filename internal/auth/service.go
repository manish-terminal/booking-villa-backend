package auth

import (
	"context"
	"fmt"

	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/users"
	"github.com/booking-villa-backend/internal/utils"
)

// AuthResult contains the result of an authentication operation.
type AuthResult struct {
	Token   string             `json:"token"`
	User    users.UserResponse `json:"user"`
	IsNew   bool               `json:"isNew"`
	Message string             `json:"message,omitempty"`
}

// Service provides authentication operations.
type Service struct {
	db          *db.Client
	otpService  *OTPService
	userService *users.Service
}

// NewService creates a new auth service.
func NewService(dbClient *db.Client) *Service {
	return &Service{
		db:          dbClient,
		otpService:  NewOTPService(dbClient),
		userService: users.NewService(dbClient),
	}
}

// SendOTPRequest represents a request to send an OTP.
type SendOTPRequest struct {
	Phone string `json:"phone"`
}

// SendOTP generates and sends an OTP to the given phone number.
func (s *Service) SendOTP(ctx context.Context, phone string) (string, error) {
	if phone == "" {
		return "", fmt.Errorf("phone number is required")
	}

	// Validate phone format (basic validation)
	if len(phone) < 10 {
		return "", fmt.Errorf("invalid phone number format")
	}

	code, err := s.otpService.SendOTP(ctx, phone)
	if err != nil {
		return "", fmt.Errorf("failed to send OTP: %w", err)
	}

	return code, nil
}

// VerifyOTPRequest represents a request to verify an OTP.
type VerifyOTPRequest struct {
	Phone      string     `json:"phone"`
	Code       string     `json:"code"`
	Name       string     `json:"name,omitempty"`       // For new users
	Role       users.Role `json:"role,omitempty"`       // Default to agent
	InviteCode string     `json:"inviteCode,omitempty"` // Property invite code
}

// VerifyOTP validates the OTP and returns an auth result.
// If the user doesn't exist, it auto-creates them.
func (s *Service) VerifyOTP(ctx context.Context, req VerifyOTPRequest) (*AuthResult, error) {
	if req.Phone == "" || req.Code == "" {
		return nil, fmt.Errorf("phone and code are required")
	}

	// Verify the OTP
	valid, err := s.otpService.VerifyOTP(ctx, req.Phone, req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to verify OTP: %w", err)
	}

	if !valid {
		return nil, fmt.Errorf("invalid or expired OTP")
	}

	// Set default role if not provided
	role := req.Role
	if !role.IsValid() {
		role = users.RoleAgent
	}

	// Set default name
	name := req.Name
	if name == "" {
		name = "User " + req.Phone[len(req.Phone)-4:]
	}

	// Get or create user
	user, isNew, err := s.userService.GetOrCreateUser(ctx, req.Phone, name, role)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create user: %w", err)
	}

	// Check if user can login (admins always can, others need approval)
	if !user.CanLogin() {
		return &AuthResult{
			User:    user.ToResponse(),
			IsNew:   isNew,
			Message: "User registration pending approval",
		}, nil
	}

	// Generate JWT token
	token, err := utils.GenerateToken(user.Phone, user.Phone, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &AuthResult{
		Token:   token,
		User:    user.ToResponse(),
		IsNew:   isNew,
		Message: "Authentication successful",
	}, nil
}

// LoginRequest represents a password login request.
type LoginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

// LoginWithPassword authenticates a user with phone and password.
func (s *Service) LoginWithPassword(ctx context.Context, req LoginRequest) (*AuthResult, error) {
	if req.Phone == "" || req.Password == "" {
		return nil, fmt.Errorf("phone and password are required")
	}

	// Get user
	user, err := s.userService.GetUserByPhone(ctx, req.Phone)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Check if user has a password
	if !user.HasPassword() {
		return nil, fmt.Errorf("password not set for this user")
	}

	// Verify password
	if !utils.VerifyPassword(user.PasswordHash, req.Password) {
		return nil, fmt.Errorf("invalid password")
	}

	// Check if user can login
	if !user.CanLogin() {
		return nil, fmt.Errorf("user account pending approval")
	}

	// Generate JWT token
	token, err := utils.GenerateToken(user.Phone, user.Phone, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &AuthResult{
		Token:   token,
		User:    user.ToResponse(),
		Message: "Login successful",
	}, nil
}

// SetPasswordRequest represents a request to set user password.
type SetPasswordRequest struct {
	Phone       string `json:"phone"`
	Password    string `json:"password"`
	OldPassword string `json:"oldPassword,omitempty"` // Required if changing password
}

// SetPassword sets or updates a user's password.
func (s *Service) SetPassword(ctx context.Context, phone, password, oldPassword string) error {
	// Get user
	user, err := s.userService.GetUserByPhone(ctx, phone)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return fmt.Errorf("user not found")
	}

	// If user already has a password, verify old password
	if user.HasPassword() && oldPassword != "" {
		if !utils.VerifyPassword(user.PasswordHash, oldPassword) {
			return fmt.Errorf("invalid old password")
		}
	}

	// Hash new password
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	return s.userService.UpdatePassword(ctx, phone, hashedPassword)
}

// RefreshToken generates a new token from a valid existing token.
func (s *Service) RefreshToken(ctx context.Context, tokenString string) (*AuthResult, error) {
	// Validate existing token
	claims, err := utils.ValidateToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Get current user state
	user, err := s.userService.GetUserByPhone(ctx, claims.Phone)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Check if user can still login
	if !user.CanLogin() {
		return nil, fmt.Errorf("user account is no longer active")
	}

	// Generate new token
	newToken, err := utils.GenerateToken(user.Phone, user.Phone, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &AuthResult{
		Token:   newToken,
		User:    user.ToResponse(),
		Message: "Token refreshed",
	}, nil
}
