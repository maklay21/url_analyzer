package gateway

import (
	"context"
	"net/http"
	"strings"

	"backend/internal/auth"

	"google.golang.org/grpc/metadata"
)

type ctxKey string

const userIDKey ctxKey = "user-id"

// HTTPAuthMiddleware проверяет JWT и прокидывает user-id в gRPC metadata
func HTTPAuthMiddleware(next http.Handler, jwtMgr *auth.JWTManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Пропускаем публичные маршруты
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"message":"missing token"}`, http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := jwtMgr.Verify(token)
		if err != nil {
			http.Error(w, `{"message":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		// Прокидываем user-id в gRPC metadata через контекст
		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GRPCMetadataAnnotator добавляет user-id в исходящие gRPC metadata
func GRPCMetadataAnnotator(ctx context.Context, r *http.Request) metadata.MD {
	if userID, ok := ctx.Value(userIDKey).(string); ok {
		return metadata.Pairs("user-id", userID)
	}
	return metadata.MD{}
}

func isPublicPath(path string) bool {
	public := []string{"/api/auth/login", "/api/auth/register", "/api/auth/refresh"}
	for _, p := range public {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
