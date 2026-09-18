package extractor

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// stubDoer возвращает заданный ответ
type stubDoer struct {
	resp *http.Response
	err  error
}

func (s *stubDoer) Do(*http.Request) (*http.Response, error) {
	return s.resp, s.err
}

func mkResp(status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

const longHTML = `<html><head><title>Test</title></head><body>
<article><h1>Заголовок</h1><p>` + // длинный текст
	"Это достаточно длинный текст для теста. " +
	"Повторяем его много раз, чтобы пройти порог в 100 символов. " +
	"Ещё немного текста, чтобы точно хватило. " +
	"И финальное предложение для надёжности.</p></article></body></html>`

func TestExtract_Success(t *testing.T) {
	e := New(&stubDoer{resp: mkResp(200, "text/html; charset=utf-8", longHTML)})

	art, err := e.Extract(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if art.Length < minContentChars {
		t.Fatalf("length too small: %d", art.Length)
	}
	if !strings.Contains(art.TextContent, "достаточно длинный текст") {
		t.Fatalf("expected text content, got: %q", art.TextContent)
	}
}

func TestExtract_InsufficientContent(t *testing.T) {
	shortHTML := `<html><body><article><p>мало</p></article></body></html>`
	e := New(&stubDoer{resp: mkResp(200, "text/html", shortHTML)})

	_, err := e.Extract(context.Background(), "https://example.com")
	if !errors.Is(err, ErrInsufficientContent) {
		t.Fatalf("expected ErrInsufficientContent, got %v", err)
	}
}

func TestExtract_EmptyBody(t *testing.T) {
	e := New(&stubDoer{resp: mkResp(200, "text/html", "   \n\t  ")})
	_, err := e.Extract(context.Background(), "https://example.com")
	if !errors.Is(err, ErrEmptyBody) {
		t.Fatalf("expected ErrEmptyBody, got %v", err)
	}
}

func TestExtract_NonHTML(t *testing.T) {
	e := New(&stubDoer{resp: mkResp(200, "application/pdf", "%PDF-1.4")})
	_, err := e.Extract(context.Background(), "https://example.com/file.pdf")
	if !errors.Is(err, ErrNonHTML) {
		t.Fatalf("expected ErrNonHTML, got %v", err)
	}
}

func TestExtract_Blocked(t *testing.T) {
	e := New(&stubDoer{resp: mkResp(403, "text/html", "forbidden")})
	_, err := e.Extract(context.Background(), "https://example.com")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked, got %v", err)
	}
}

func TestExtract_HTTPStatus
