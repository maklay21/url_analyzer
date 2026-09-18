package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Task struct {
	ID           string
	UserID       string
	URL          string
	Domain       string
	Status       string
	Summary      string
	FilePath     string
	ErrorMessage string
	CreatedAt    time.Time
}

type Storage struct {
	db        *pgxpool.Pool
	outputDir string
}

func New(db *pgxpool.Pool, outputDir string) *Storage {
	_ = os.MkdirAll(outputDir, 0o755)
	return &Storage{db: db, outputDir: outputDir}
}

// generateTaskID — url + uuid.uuid4() из задания
func (s *Storage) generateFileName(rawURL string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	uuidStr := hex.EncodeToString(b)

	// Очищаем URL от недопустимых символов для имени файла
	safeURL := strings.NewReplacer(
		"/", "_", ":", "_", "?", "_", "&", "_",
		"=", "_", "#", "_", " ", "_",
	).Replace(rawURL)

	return fmt.Sprintf("%s_%s.txt", safeURL, uuidStr)
}

func (s *Storage) CreateTask(ctx context.Context, userID, rawURL string) (*Task, error) {
	domain := extractDomain(rawURL)
	taskID := generateID()

	_, err := s.db.Exec(ctx, `
		INSERT INTO tasks (id, user_id, url, domain, status, created_at)
		VALUES ($1, $2, $3, $4, 'processing', NOW())
	`, taskID, userID, rawURL, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	return &Task{
		ID:     taskID,
		UserID: userID,
		URL:    rawURL,
		Domain: domain,
		Status: "processing",
	}, nil
}

func (s *Storage) CompleteTask(ctx context.Context, taskID, summary string) (string, error) {
	// Получаем URL задачи для имени файла
	var rawURL string
	err := s.db.QueryRow(ctx, `SELECT url FROM tasks WHERE id = $1`, taskID).Scan(&rawURL)
	if err != nil {
		return "", err
	}

	// Создаём файл: {url}_{uuid}.txt
	fileName := s.generateFileName(rawURL)
	filePath := filepath.Join(s.outputDir, fileName)

	if err := os.WriteFile(filePath, []byte(summary), 0o644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	_, err = s.db.Exec(ctx, `
		UPDATE tasks SET status = 'completed', summary = $1, file_path = $2
		WHERE id = $3
	`, summary, filePath, taskID)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

func (s *Storage) FailTask(ctx context.Context, taskID, errMsg string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE tasks SET status = 'failed', error_message = $1 WHERE id = $2
	`, errMsg, taskID)
	return err
}

func (s *Storage) SetInsufficientData(ctx context.Context, taskID string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE tasks SET status = 'insufficient_data', summary = 'Недостаточно данных для анализа'
		WHERE id = $1
	`, taskID)
	return err
}

func (s *Storage) GetTask(ctx context.Context, taskID string) (*Task, error) {
	t := &Task{}
	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, url, domain, status, COALESCE(summary, ''), 
		       COALESCE(file_path, ''), COALESCE(error_message, ''), created_at
		FROM tasks WHERE id = $1
	`, taskID).Scan(&t.ID, &t.UserID, &t.URL, &t.Domain, &t.Status,
		&t.Summary, &t.FilePath, &t.ErrorMessage, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Storage) ListTasks(ctx context.Context, userID string, page, pageSize int) ([]Task, int, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM tasks WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, url, domain, status, COALESCE(summary, ''), created_at
		FROM tasks WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.UserID, &t.URL, &t.Domain,
			&t.Status, &t.Summary, &t.CreatedAt); err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, t)
	}

	return tasks, total, nil
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func extractDomain(rawURL string) string {
	for i := 0; i < len(rawURL); i++ {
		if i+3 <= len(rawURL) && rawURL[i:i+3] == "://" {
			rest := rawURL[i+3:]
			for j := 0; j < len(rest); j++ {
				if rest[j] == '/' {
					return rest[:j]
				}
			}
			return rest
		}
	}
	return rawURL
}
