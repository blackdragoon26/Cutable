package httpapi

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	if _, err := mail.ParseAddress(input.Email); err != nil || len(input.Password) < 8 || len(input.Name) > 100 {
		writeError(w, http.StatusBadRequest, "valid email and password of at least 8 characters are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	user, err := s.store.CreateUser(r.Context(), input.Name, input.Email, string(hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "user already exists")
			return
		}
		s.internalError(w, r, err)
		return
	}
	token, err := s.issueAuthToken(w, user.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "User created successfully", "token": token})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := s.store.UserByEmail(r.Context(), input.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := s.issueAuthToken(w, user.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Login successful", "token": token})
}

func (s *Server) logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: authCookieSameSite(s.cfg.CookieSecure),
	})
	writeJSON(w, http.StatusOK, map[string]any{"message": "Logged out"})
}

func (s *Server) currentUser(w http.ResponseWriter, r *http.Request) {
	user, err := s.store.UserByID(r.Context(), userID(r.Context()))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		},
	})
}
