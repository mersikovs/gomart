package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mersikovs/gomart/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrInvalidCredentials = errors.New("invalid login or password pair")

type UserService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
	GetBalance(ctx context.Context, userId int64) (*BalanceResponse, error)
}

type userService struct {
	repo       repository.Storage
	jwtSecret  string
	bcryptCost int
	logger     *slog.Logger
}

type BalanceResponse struct {
	CurrentBalance float64 `json:"current"`
	TotalSpent     float64 `json:"withdrawn"`
}

func NewUserService(repo repository.Storage, secret string, bCost int) UserService {
	return &userService{
		repo:       repo,
		jwtSecret:  secret,
		bcryptCost: bCost,
	}
}

func (s *userService) Register(ctx context.Context, login, password string) (string, error) {
	_, err := s.repo.FindUserByLogin(ctx, login)
	switch {
	case err == nil:
		return "", ErrUserAlreadyExists
	case errors.Is(err, repository.ErrUserNotFound):
	default:
		return "", fmt.Errorf("register: find user by login: %w", err)
	}

	cost := bcrypt.DefaultCost
	if s.bcryptCost > 0 {
		cost = s.bcryptCost
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}

	userId, err := s.repo.CreateUser(ctx, login, string(hashedPassword))
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return "", ErrUserAlreadyExists
		}
		return "", err
	}

	token, err := s.generateJWT(userId, login)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *userService) Login(ctx context.Context, login, password string) (string, error) {

	user, err := s.repo.FindUserByLogin(ctx, login)
	if err != nil {
		return "", fmt.Errorf("register: find user by login: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", ErrInvalidCredentials
	}

	token, err := s.generateJWT(user.ID, login)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *userService) GetBalance(ctx context.Context, userId int64) (*BalanceResponse, error) {
	user, err := s.repo.FindUserByID(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("error GetOrdersByUser: %w", err)
	}

	return &BalanceResponse{
		CurrentBalance: float64(user.Balance),
		TotalSpent:     float64(user.TotalSpent),
	}, nil
}

func (s *userService) generateJWT(userID int64, username string) (string, error) {
	claims := jwt.MapClaims{
		"userId":   userID,
		"userName": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}
