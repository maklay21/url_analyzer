package auth

import (
	"encoding/json"
	"net/http"
	"time"
)

type HTTPHandler struct {
	repo *Repository
	jwt  *JWTManager
}

func NewHTTPHandler(jwt *JWTManager, repo *Repository) *HTTPHandler {
	return &HTTPHandler{jwt: jwt, repo: repo}
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/register":
		h.register(w, r)
	case "/login":
		h.login(w, r)
	case "/refresh":
		h.refresh(w, r)
	case "/logout":
		h.logout(w, r)
	case "/me":
		h.me(w, r)
	default:
		http.NotFound(w, r)
	}
}

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *HTTPHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "password too short")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash failed")
		return
	}

	user, err := h.repo.CreateUser(r.Context(), req.Email, hash)
	if err != nil {
		if err == ErrUserExists {
			writeErr(w, http.StatusConflict, "user exists")
			return
		}
		writeErr(w, http.StatusInternalServerError, "create failed")
		return
	}

	h.issueTokens(w, r, user.ID)
}

func (h *HTTPHandler) login(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil || !CheckPassword(user.PasswordHash, req.Password) {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	h.issueTokens(w, r, user.ID)
}

func (h *HTTPHandler) issueTokens(w http.ResponseWriter, r *http.Request, userID string) {
	ctx := r.Context()

	access, err := h.jwt.GenerateAccess(userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "jwt failed")
		return
	}

	rawRefresh, refreshHash, err := GenerateRefresh()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "refresh failed")
		return
	}

	expiresAt := time.Now().Add(h.jwt.RefreshTTL())
	if err := h.repo.SaveRefresh(ctx, userID, refreshHash, expiresAt); err != nil {
		writeErr(w, http.StatusInternalServerError, "save refresh failed")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    rawRefresh,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"accessToken": access,
		"userId":      userID,
	})
}

func (h *HTTPHandler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "no refresh token")
		return
	}

	userID, err := h.repo.ConsumeRefresh(r.Context(), HashToken(cookie.Value))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	h.issueTokens(w, r, userID)
}

func (h *HTTPHandler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		if userID, err := h.repo.ConsumeRefresh(r.Context(), HashToken(cookie.Value)); err == nil {
			_ = h.repo.RevokeAllForUser(r.Context(), userID)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "refresh_token",
		Value:  "",
		Path:   "/api/auth",
		MaxAge: -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HTTPHandler) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"message": msg})
}
