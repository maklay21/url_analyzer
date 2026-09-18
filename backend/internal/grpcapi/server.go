package grpcapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"backend/internal/extractor"
	"backend/internal/ollama"
	"backend/internal/storage"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Server реализует AnalyzerServiceServer.
type Server struct {
	UnimplementedAnalyzerServiceServer

	storage   *storage.Storage
	extractor *extractor.Extractor
	ollama    *ollama.Client
}

// NewServer собирает зависимости. Если extractor == nil —
// создаётся дефолтный с http.Client.
func NewServer(st *storage.Storage, ex *extractor.Extractor, oc *ollama.Client) *Server {
	if ex == nil {
		ex = extractor.New(nil)
	}
	return &Server{
		storage:   st,
		extractor: ex,
		ollama:    oc,
	}
}

// --- Interceptor авторизации ---

type ctxKey string

const userIDKey ctxKey = "user-id"

func AuthInterceptor(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	ids := md.Get("user-id")
	if len(ids) == 0 || ids[0] == "" {
		return nil, status.Error(codes.Unauthenticated, "user-id required")
	}

	ctx = context.WithValue(ctx, userIDKey, ids[0])
	return handler(ctx, req)
}

func userIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

// --- RPC методы ---

func (s *Server) Analyze(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResponse, error) {
	if req.Url == "" {
		return nil, status.Error(codes.InvalidArgument, "url required")
	}

	userID := userIDFromCtx(ctx)
	if userID == "" {
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}

	task, err := s.storage.CreateTask(ctx, userID, req.Url)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create task: %v", err)
	}

	// Асинхронная обработка. Контекст — собственный, не привязан к запросу.
	go s.processTask(context.Background(), task.ID, req.Url)

	return &AnalyzeResponse{
		TaskId: task.ID,
		Status: "processing",
	}, nil
}

func (s *Server) processTask(ctx context.Context, taskID, rawURL string) {
	article, err := s.extractor.Extract(ctx, rawURL)
	if err != nil {
		slog.Warn("extract failed", "task_id", taskID, "err", err)

		switch {
		case extractor.IsInsufficient(err):
			_ = s.storage.SetInsufficientData(ctx, taskID)

		case errors.Is(err, extractor.ErrBlocked):
			_ = s.storage.FailTask(ctx, taskID, "сайт блокирует автоматический доступ")

		case errors.Is(err, extractor.ErrNonHTML):
			_ = s.storage.FailTask(ctx, taskID, "контент не является HTML-страницей")

		case errors.Is(err, extractor.ErrHTTPStatus):
			var ee *extractor.ExtractError
			_ = errors.As(err, &ee)
			_ = s.storage.FailTask(ctx, taskID,
				fmt.Sprintf("сайт вернул статус %d", ee.Code))

		default:
			_ = s.storage.FailTask(ctx, taskID, err.Error())
		}
		return
	}

	summary, err := s.ollama.GenerateSummary(ctx, article.TextContent)
	if err != nil {
		slog.Error("ollama failed", "task_id", taskID, "err", err)
		_ = s.storage.FailTask(ctx, taskID, err.Error())
		return
	}

	if isInsufficientFromLLM(summary) {
		_ = s.storage.SetInsufficientData(ctx, taskID)
		return
	}

	if _, err := s.storage.CompleteTask(ctx, taskID, summary); err != nil {
		slog.Error("save result failed", "task_id", taskID, "err", err)
		_ = s.storage.FailTask(ctx, taskID, err.Error())
	}
}

func (s *Server) GetTask(ctx context.Context, req *GetTaskRequest) (*AnalyzeResponse, error) {
	task, err := s.storage.GetTask(ctx, req.TaskId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "task not found: %v", err)
	}
	return &AnalyzeResponse{
		TaskId:       task.ID,
		Status:       task.Status,
		Summary:      task.Summary,
		FilePath:     task.FilePath,
		ErrorMessage: task.ErrorMessage,
	}, nil
}

func (s *Server) ListTasks(ctx context.Context, req *ListTasksRequest) (*ListTasksResponse, error) {
	userID := userIDFromCtx(ctx)
	if userID == "" {
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}

	tasks, total, err := s.storage.ListTasks(ctx, userID, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tasks: %v", err)
	}

	items := make([]*TaskItem, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, &TaskItem{
			TaskId:    t.ID,
			Url:       t.URL,
			Domain:    t.Domain,
			Status:    t.Status,
			Summary:   t.Summary,
			CreatedAt: t.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &ListTasksResponse{Tasks: items, Total: int32(total)}, nil
}

// --- вспомогательное ---

func isInsufficientFromLLM(s string) bool {
	return contains(s, "Недостаточно данных для анализа") ||
		contains(s, "insufficient data") ||
		contains(s, "недостаточно")
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
