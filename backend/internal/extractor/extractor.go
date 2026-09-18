package extractor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

const (
	// Минимальная длина текста, ниже которой считаем, что данных мало
	minContentChars = 100

	// Лимит на размер скачиваемого тела — защита от «бесконечных» страниц
	maxBodyBytes = 5 << 20 // 5 MiB

	// User-Agent, чтобы не отдавали 403 на пустой UA
	userAgent = "Mozilla/5.0 (compatible; URLAnalyzer/1.0)"
)

// Article — результат извлечения основного контента страницы
type Article struct {
	Title       string
	TextContent string
	Excerpt     string
	Byline      string
	SiteName    string
	Language    string
	Length      int
}

// Doer абстрагирует http.Client, чтобы подменять его в тестах
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Extractor скачивает страницу и извлекает основной текст
type Extractor struct {
	client Doer
}

// Cоздаём Extractor. Если client == nil — используется http.Client с таймаутом 30 секунд
func New(client Doer) *Extractor {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Extractor{client: client}
}

// Extract скачивает страницу по rawURL и возвращает Article
// При любой проблеме возвращает *ExtractError с одной из sentinel-ошибок
func (e *Extractor) Extract(ctx context.Context, rawURL string) (*Article, error) {
	// 1. Валидация URL
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return nil, &ExtractError{Kind: ErrFetchFailed, Msg: "invalid url"}
	}

	// 2. Формируем запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &ExtractError{Kind: ErrFetchFailed, Msg: err.Error()}
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	// 3. Скачиваем
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, &ExtractError{Kind: ErrFetchFailed, Msg: err.Error()}
	}
	defer resp.Body.Close()

	// 4. Проверяем статус
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, &ExtractError{Kind: ErrBlocked, Code: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ExtractError{Kind: ErrHTTPStatus, Code: resp.StatusCode}
	}

	// 5. Проверяем Content-Type
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "text/html") &&
		!strings.Contains(ct, "application/xhtml") {
		return nil, &ExtractError{Kind: ErrNonHTML, Msg: ct}
	}

	// 6. Читаем тело с лимитом
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, &ExtractError{Kind: ErrFetchFailed, Msg: err.Error()}
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, &ExtractError{Kind: ErrEmptyBody}
	}

	// 7. Парсим URL (для readability нужен *url.URL)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, &ExtractError{Kind: ErrFetchFailed, Msg: err.Error()}
	}

	// 8. Извлекаем контент через readability
	article, err := readability.FromReader(strings.NewReader(string(body)), parsed)
	if err != nil {
		return nil, &ExtractError{Kind: ErrParseFailed, Msg: err.Error()}
	}

	// 9. Проверяем, что текста достаточно
	text := strings.TrimSpace(article.TextContent)
	if len(text) < minContentChars {
		return nil, &ExtractError{
			Kind: ErrInsufficientContent,
			Msg:  fmt.Sprintf("only %d chars", len(text)),
		}
	}

	return &Article{
		Title:       article.Title,
		TextContent: text,
		Excerpt:     article.Excerpt,
		Byline:      article.Byline,
		SiteName:    article.SiteName,
		Language:    article.Language,
		Length:      len(text),
	}, nil
}
