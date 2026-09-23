package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mersikovs/gomart/internal/model"
	"github.com/mersikovs/gomart/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// ErrUserAlreadyExists возвращается при попытке регистрации пользователя
// с уже занятым логином или идентификатором.
var ErrUserAlreadyExists = errors.New("user already exists")

// ErrInvalidCredentials возвращается при ошибке аутентификации,
// если переданная пара логин-пароль не найдена в системе.
var ErrInvalidCredentials = errors.New("invalid login or password pair")

// UserService определяет контракт бизнес-логики для управления пользователями и их балансами.
// Все методы принимают context.Context для поддержки отмены операций, соблюдения таймаутов
// и передачи JWT-токена.
type UserService interface {
	// Register создает нового пользователя в системе.
	// Принимает логин и пароль. Возвращает Token JWT.
	// В случае конфликта (попытка регистрации существующего логина) возвращает ошибку ErrUserAlreadyExists.
	Register(ctx context.Context, login, password string) (string, error)

	// Login выполняет аутентификацию пользователя по паре логин-пароль.
	// В случае успеха возвращает токен (JWT)
	// Если данные неверны, возвращает ошибку ErrInvalidCredentials.
	Login(ctx context.Context, login, password string) (string, error)

	// GetBalance запрашивает актуальное состояние счета пользователя по его внутреннему идентификатору.
	// Возвращает структуру с деталями баланса.
	GetBalance(ctx context.Context, userID int64) (*model.User, error)
}

type userService struct {
	repo       repository.Storage
	jwtSecret  string
	bcryptCost int
}

// NewUserService конструктор userService, который возвращает сконфигурированный указатель на структуру
func NewUserService(repo repository.Storage, secret string, bCost int) UserService {
	return &userService{
		repo:       repo,
		jwtSecret:  secret,
		bcryptCost: bCost,
	}
}

func (s *userService) Register(ctx context.Context, login, password string) (string, error) {
	cost := bcrypt.DefaultCost
	if s.bcryptCost > 0 {
		cost = s.bcryptCost
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}

	userID, err := s.repo.CreateUser(ctx, login, string(hashedPassword))
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return "", ErrUserAlreadyExists
		}
		return "", err
	}

	token, err := s.generateJWT(userID, login)
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

func (s *userService) GetBalance(ctx context.Context, userID int64) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error GetOrdersByUser: %w", err)
	}

	return user, nil
}

func (s *userService) generateJWT(userID int64, username string) (string, error) {
	claims := jwt.MapClaims{
		"userID":   userID,
		"userName": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}
