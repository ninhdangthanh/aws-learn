package service

import (
	"context"
	"errors"
	"time"

	"github.com/go-template/database"
	"github.com/go-template/elastic"
	"github.com/go-template/messaging"
	"github.com/go-template/models"
	"github.com/go-template/redis"
	"github.com/go-template/utils"
	"golang.org/x/crypto/bcrypt"
)

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type UserService interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUser(ctx context.Context, id uint) (*models.User, error)
	Login(ctx context.Context, email, password string, secret string, accessExp, refreshExp int) (*AuthResponse, error)
	RefreshToken(ctx context.Context, refreshToken string, secret string, accessExp int) (*AuthResponse, error)
	Logout(ctx context.Context, userID uint, accessJTI, refreshJTI string) error
	EvictUser(ctx context.Context, userID uint) error
}

type userService struct{}

func NewUserService() UserService {
	return &userService{}
}

func (s *userService) CreateUser(ctx context.Context, user *models.User) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hashedPassword)
	if err := database.CreateUser(user); err != nil {
		return err
	}
	_ = elastic.IndexUser(user)
	_ = redis.CacheUser(user)
	_ = messaging.PublishUserCreatedEvent(user)
	return nil
}

func (s *userService) GetUser(ctx context.Context, id uint) (*models.User, error) {
	if user, err := redis.GetCachedUser(id); err == nil && user != nil {
		return user, nil
	}
	user, err := database.GetUser(id)
	if err != nil {
		return nil, err
	}
	_ = redis.CacheUser(user)
	return user, nil
}

func (s *userService) Login(ctx context.Context, email, password string, secret string, accessExp, refreshExp int) (*AuthResponse, error) {
	user, err := database.GetUserByEmail(email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Generate Access Token
	accToken, accJti, _ := utils.GenerateToken(user.ID, "access", secret, time.Duration(accessExp)*time.Hour)
	_ = redis.SetSession(user.ID, accJti, time.Duration(accessExp)*time.Hour)

	// Generate Refresh Token
	refToken, refJti, _ := utils.GenerateToken(user.ID, "refresh", secret, time.Duration(refreshExp)*24*time.Hour)
	_ = redis.SetSession(user.ID, refJti, time.Duration(refreshExp)*24*time.Hour)

	return &AuthResponse{AccessToken: accToken, RefreshToken: refToken}, nil
}

func (s *userService) RefreshToken(ctx context.Context, refreshToken string, secret string, accessExp int) (*AuthResponse, error) {
	claims, err := utils.ValidateToken(refreshToken, secret)
	if err != nil || claims.Type != "refresh" {
		return nil, errors.New("invalid refresh token")
	}

	if !redis.IsSessionValid(claims.UserID, claims.JTI) {
		return nil, errors.New("session revoked")
	}

	// Generate New Access Token
	accToken, accJti, _ := utils.GenerateToken(claims.UserID, "access", secret, time.Duration(accessExp)*time.Hour)
	_ = redis.SetSession(claims.UserID, accJti, time.Duration(accessExp)*time.Hour)

	return &AuthResponse{AccessToken: accToken, RefreshToken: refreshToken}, nil
}

func (s *userService) Logout(ctx context.Context, userID uint, accessJTI, refreshJTI string) error {
	_ = redis.RevokeSession(userID, accessJTI)
	_ = redis.RevokeSession(userID, refreshJTI)
	return nil
}

func (s *userService) EvictUser(ctx context.Context, userID uint) error {
	return redis.EvictUser(userID)
}
