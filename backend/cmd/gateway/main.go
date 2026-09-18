package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"backend/internal/auth"
	"backend/internal/gateway"
	gw "backend/internal/grpcapi"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx := context.Background()

	// --- JWT ---
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "change-me-in-prod"
	}
	jwtMgr := auth.NewJWTManager(jwtSecret, 15*time.Minute, 7*24*time.Hour)

	// --- Postgres ---
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/analyzer?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// --- Auth repo + handler ---
	authRepo := auth.NewRepository(pool)
	authHandler := auth.NewHTTPHandler(jwtMgr, authRepo)

	// --- gRPC connection ---
	grpcAddr := os.Getenv("GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = "localhost:50051"
	}
	conn, err := grpc.DialContext(ctx, grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		slog.Error("dial grpc", "err", err)
		os.Exit(1)
	}

	// --- gRPC-Gateway mux ---
	mux := runtime.NewServeMux(
		runtime.WithMetadata(gateway.GRPCMetadataAnnotator),
	)

	if err := gw.RegisterAnalyzerServiceHandler(ctx, mux, conn); err != nil {
		slog.Error("register gateway", "err", err)
		os.Exit(1)
	}

	// --- Router ---
	root := http.NewServeMux()
	root.Handle("/api/auth/", http.StripPrefix("/api/auth", authHandler))
	root.Handle("/", gateway.HTTPAuthMiddleware(mux, jwtMgr))

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      corsMiddleware(root),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
	}

	slog.Info("HTTP gateway on :8080")
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
	}
}

// corsMiddleware разрешает запросы с фронтенда (Vite на :5173).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
