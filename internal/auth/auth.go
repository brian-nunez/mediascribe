package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/brian-nunez/video-to-blog-page/internal/db"
)

type Service struct {
	Store      *db.Store
	SessionTTL time.Duration
	CookieName string
}

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrUnauthorized = errors.New("unauthorized")

func HashPassword(password string) (string, error) {
	p := strings.TrimSpace(password)
	if len(p) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}
	out, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (s Service) CreateAdminUser(ctx context.Context, username, password string) (db.AdminUser, error) {
	u := strings.TrimSpace(strings.ToLower(username))
	if u == "" {
		return db.AdminUser{}, errors.New("username is required")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return db.AdminUser{}, err
	}
	out := db.AdminUser{
		ID:           uuid.NewString(),
		Username:     u,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.Store.CreateAdminUser(ctx, out); err != nil {
		return db.AdminUser{}, err
	}
	out.PasswordHash = ""
	return out, nil
}

func (s Service) Login(ctx context.Context, username, password string) (string, db.AdminUser, error) {
	u := strings.TrimSpace(strings.ToLower(username))
	user, err := s.Store.GetAdminUserByUsername(ctx, u)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return "", db.AdminUser{}, ErrInvalidCredentials
		}
		return "", db.AdminUser{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(strings.TrimSpace(password))); err != nil {
		return "", db.AdminUser{}, ErrInvalidCredentials
	}

	token, err := newSessionToken()
	if err != nil {
		return "", db.AdminUser{}, err
	}
	session := db.AdminSession{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: hashToken(token),
		ExpiresAt: time.Now().UTC().Add(s.SessionTTL),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Store.CreateAdminSession(ctx, session); err != nil {
		return "", db.AdminUser{}, err
	}
	user.PasswordHash = ""
	return token, user, nil
}

func (s Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.Store.RevokeAdminSessionByTokenHash(ctx, hashToken(token))
}

func (s Service) RequireUser(ctx context.Context, token string) (db.AdminUser, error) {
	if strings.TrimSpace(token) == "" {
		return db.AdminUser{}, ErrUnauthorized
	}
	session, err := s.Store.GetAdminSessionByTokenHash(ctx, hashToken(token))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return db.AdminUser{}, ErrUnauthorized
		}
		return db.AdminUser{}, err
	}
	if session.RevokedAt != "" || session.ExpiresAt.Before(time.Now().UTC()) {
		return db.AdminUser{}, ErrUnauthorized
	}
	user, err := s.Store.GetAdminUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return db.AdminUser{}, ErrUnauthorized
		}
		return db.AdminUser{}, err
	}
	user.PasswordHash = ""
	return user, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
