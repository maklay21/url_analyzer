package main

import (
	"context"
	"log/slog"
	"net"
	"os"

        "backend/internal/extractor"
	"backend/internal/grpcapi"
	"backend/internal/ollama"
	"backend/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	ctx := context.Background()

	// Postgres
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

	// Redis (для кэша — можно расширить)
	// ...

	// Ollama
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	ollamaModel := os.Getenv("OLLAMA_MODEL")
	if ollamaModel == "" {
		ollamaModel = "mistral:7b"
	}

	// Storage
	outputDir := os.Getenv("OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "./results"
	}
	st := storage.New(pool, outputDir)
        ex := extractor.New(nil)
	oc := ollama.New(ollamaURL, ollamaModel)

	// gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcapi.AuthInterceptor),
	)
	grpcapi.RegisterAnalyzerServiceServer(grpcServer, grpcapi.NewServer(st, ex, oc))
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}

	slog.Info("gRPC server listening on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
