package extractor

import "errors"

// Ошибки извлечения
var (
	// ErrInsufficientContent — контент есть, но его слишком мало для LLM
	ErrInsufficientContent = errors.New("insufficient content")

	// ErrEmptyBody — страница вернула пустое тело
	ErrEmptyBody = errors.New("empty response body")

	// ErrNonHTML — контент не HTML (PDF, картинка, бинарь)
	ErrNonHTML = errors.New("content is not HTML")

	// ErrFetchFailed — сетевая ошибка / таймаут / DNS / неверный URL
	ErrFetchFailed = errors.New("fetch failed")

	// ErrHTTPStatus — сервер вернул 4xx/5xx
	ErrHTTPStatus = errors.New("non-2xx status")

	// ErrParseFailed — HTML скачался, но парсер упал
	ErrParseFailed = errors.New("parse failed")

	// ErrBlocked — robots.txt / cloudflare / captcha / 403 / 429
	ErrBlocked = errors.New("access blocked")
)

// ExtractError оборачивает одну из sentinel-ошибок выше, добавляя контекст (сообщение и HTTP-код, если применимо)
type ExtractError struct {
	Kind error
	Msg  string
	Code int
}

func (e *ExtractError) Error() string {
	switch {
	case e.Msg != "" && e.Code != 0:
		return e.Kind.Error() + ": " + e.Msg
	case e.Msg != "":
		return e.Kind.Error() + ": " + e.Msg
	case e.Code != 0:
		return e.Kind.Error()
	default:
		return e.Kind.Error()
	}
}

// Unwrap позволяет errors.Is / errors.As работать с Kind
func (e *ExtractError) Unwrap() error { return e.Kind }

// IsInsufficient — проверка для вызывающего кода: «недостаточно данных» — это не сбой, а валидный статус задачи
func IsInsufficient(err error) bool {
	return errors.Is(err, ErrInsufficientContent) || errors.Is(err, ErrEmptyBody)
}
