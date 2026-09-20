package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userClaimsKey = contextKey("userClaims")

// AuthConfig содержит конфигурацию для middleware проверки JWT-токенов.
type AuthConfig struct {
	SecretKey []byte
}

// Auth — фабрика middleware для аутентификации через JWT.
// Извлекает токен из заголовка Authorization: Bearer <token>, проверяет его подпись и срок действия.
// В случае успеха сохраняет распарсенные Claims в контекст запроса и передает управление дальше.
// При ошибке прерывает цепочку обработчиков и возвращает 401 Unauthorized с текстовым описанием проблемы.
func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "authorization header is required", http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				http.Error(w, "invalid authorization format", http.StatusUnauthorized)
				return
			}

			var claims jwt.MapClaims
			token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return cfg.SecretKey, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaimsFromContext извлекает утверждения (Claims) авторизованного пользователя из контекста.
// Возвращает nil, если пользователь не прошел через middleware Auth или тип данных неверен.
func GetClaimsFromContext(ctx context.Context) jwt.MapClaims {
	claims, ok := ctx.Value(userClaimsKey).(jwt.MapClaims)
	if !ok {
		return nil
	}
	return claims
}

func WithTestClaims(ctx context.Context, c jwt.MapClaims) context.Context {
	return context.WithValue(ctx, userClaimsKey, c)
}
