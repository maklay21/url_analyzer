package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidToken = errors.New("invalid refresh token")
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	id := newID()
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`,
		id, email, passwordHash)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrUserExists
		}
		return nil, err
	}
	return &User{ID: id, Email: email, PasswordHash: passwordHash}, nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

func (r *Repository) SaveRefresh(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		newID(), userID, tokenHash, expiresAt)
	return err
}

// ConsumeRefresh проверяет токен, отзывает старый и возвращает user_id
func (r *Repository) ConsumeRefresh(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	var revoked bool
	var expiresAt time.Time
	err := r.db.QueryRow(ctx,
		`SELECT user_id, revoked, expires_at FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&userID, &revoked, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidToken
	}
	if err != nil {
		return "", err
	}
	if revoked || time.Now().After(expiresAt) {
		return "", ErrInvalidToken
	}
	// Ротация: отзываем старый
	_, _ = r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked = TRUE WHERE token_hash = $1`, tokenHash)
	return userID, nil
}

func (r *Repository) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1`, userID)
	return err
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isUniqueViolation(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate") || contains(err.Error(), "unique"))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
