package service

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-localbase/internal/auth"
	"ai-localbase/internal/model"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	// ErrUserNotFound is returned when a lookup misses.
	ErrUserNotFound = errors.New("user not found")
	// ErrUsernameTaken is returned when registering an existing username.
	ErrUsernameTaken = errors.New("username already taken")
	// ErrSessionNotFound is returned when a refresh-session lookup misses.
	ErrSessionNotFound = errors.New("session not found")
)

// UserStore manages users and refresh-token sessions in SQLite.
type UserStore struct {
	db            *sql.DB
	encryptionKey []byte
}

// NewUserStore opens (or creates) the auth SQLite file and ensures schema.
// encryptionKey must be 32 bytes (use auth.DeriveConfigEncryptionKey).
// Pass nil to disable at-rest encryption (testing only).
func NewUserStore(path string, encryptionKey []byte) (*UserStore, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("auth sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(trimmed), 0o755); err != nil {
		return nil, fmt.Errorf("create auth db directory: %w", err)
	}
	db, err := sql.Open("sqlite", trimmed)
	if err != nil {
		return nil, fmt.Errorf("open auth sqlite: %w", err)
	}
	store := &UserStore{db: db, encryptionKey: encryptionKey}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *UserStore) init() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS auth_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			refresh_token_hash TEXT NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id ON auth_sessions(user_id)`,
		`CREATE TABLE IF NOT EXISTS user_configs (
			user_id TEXT PRIMARY KEY,
			chat_provider TEXT NOT NULL DEFAULT '',
			chat_base_url TEXT NOT NULL DEFAULT '',
			chat_model TEXT NOT NULL DEFAULT '',
			chat_api_key TEXT NOT NULL DEFAULT '',
			chat_temperature REAL NOT NULL DEFAULT 0.7,
			chat_context_limit INTEGER NOT NULL DEFAULT 12,
			chat_extra_headers TEXT NOT NULL DEFAULT '',
			embedding_provider TEXT NOT NULL DEFAULT '',
			embedding_base_url TEXT NOT NULL DEFAULT '',
			embedding_model TEXT NOT NULL DEFAULT '',
			embedding_api_key TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init auth schema: %w", err)
		}
	}
	if _, err := s.db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}
	// Idempotent column adds (SQLite has no IF NOT EXISTS for ADD COLUMN).
	if err := s.addColumnIfMissing("user_configs", "chat_extra_headers", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

// addColumnIfMissing inspects PRAGMA table_info and runs ALTER TABLE only when needed.
func (s *UserStore) addColumnIfMissing(table, column, definition string) error {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

// Close releases the database handle.
func (s *UserStore) Close() error {
	return s.db.Close()
}

// CreateUser inserts a new user, returning the created row.
// Returns ErrUsernameTaken when the username already exists.
func (s *UserStore) CreateUser(username, passwordHash string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if passwordHash == "" {
		return nil, fmt.Errorf("password hash is required")
	}
	user := &model.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC(),
	}
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		user.ID, user.Username, user.PasswordHash, user.CreatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return user, nil
}

// GetUserByUsername returns the user row or ErrUserNotFound.
func (s *UserStore) GetUserByUsername(username string) (*model.User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`,
		strings.TrimSpace(username),
	)
	return scanUser(row)
}

// GetUserByID returns the user row or ErrUserNotFound.
func (s *UserStore) GetUserByID(id string) (*model.User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, created_at FROM users WHERE id = ?`,
		strings.TrimSpace(id),
	)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*model.User, error) {
	u := &model.User{}
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

// CreateSession records a new refresh-token session.
func (s *UserStore) CreateSession(userID, refreshTokenHash string, ttl time.Duration) (*model.AuthSession, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if strings.TrimSpace(refreshTokenHash) == "" {
		return nil, fmt.Errorf("refresh token hash is required")
	}
	now := time.Now().UTC()
	session := &model.AuthSession{
		ID:               uuid.NewString(),
		UserID:           userID,
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        now.Add(ttl),
		CreatedAt:        now,
	}
	_, err := s.db.Exec(
		`INSERT INTO auth_sessions (id, user_id, refresh_token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.RefreshTokenHash, session.ExpiresAt, session.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return session, nil
}

// GetSessionByRefreshTokenHash looks up an active session and returns the
// associated user. Expired sessions return ErrSessionNotFound.
func (s *UserStore) GetSessionByRefreshTokenHash(hash string) (*model.AuthSession, *model.User, error) {
	row := s.db.QueryRow(
		`SELECT s.id, s.user_id, s.refresh_token_hash, s.expires_at, s.created_at,
		        u.id, u.username, u.password_hash, u.created_at
		 FROM auth_sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.refresh_token_hash = ?`,
		hash,
	)
	sess := &model.AuthSession{}
	user := &model.User{}
	err := row.Scan(
		&sess.ID, &sess.UserID, &sess.RefreshTokenHash, &sess.ExpiresAt, &sess.CreatedAt,
		&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrSessionNotFound
		}
		return nil, nil, fmt.Errorf("scan session: %w", err)
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.deleteSessionByHash(hash)
		return nil, nil, ErrSessionNotFound
	}
	return sess, user, nil
}

// DeleteSessionByHash removes a session matching the hash. No-op if not present.
func (s *UserStore) DeleteSessionByHash(hash string) error {
	return s.deleteSessionByHash(hash)
}

func (s *UserStore) deleteSessionByHash(hash string) error {
	_, err := s.db.Exec(`DELETE FROM auth_sessions WHERE refresh_token_hash = ?`, hash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions purges all sessions past their expiry. Returns row count.
func (s *UserStore) DeleteExpiredSessions() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM auth_sessions WHERE expires_at < ?`, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// --- User config CRUD ---

// GetUserConfig returns (chat, embedding, true) when a row exists.
// Returns (zero, zero, false) when the user has no saved config yet.
// API keys are decrypted with the store's master key; rows written before
// encryption was enabled are returned as plaintext (auth.DecryptString passthrough).
func (s *UserStore) GetUserConfig(userID string) (model.ChatConfig, model.EmbeddingConfig, bool, error) {
	uid := strings.TrimSpace(userID)
	if uid == "" {
		return model.ChatConfig{}, model.EmbeddingConfig{}, false, fmt.Errorf("user id is required")
	}
	row := s.db.QueryRow(`
		SELECT chat_provider, chat_base_url, chat_model, chat_api_key,
		       chat_temperature, chat_context_limit, chat_extra_headers,
		       embedding_provider, embedding_base_url, embedding_model, embedding_api_key
		FROM user_configs
		WHERE user_id = ?`, uid)
	var chat model.ChatConfig
	var emb model.EmbeddingConfig
	err := row.Scan(
		&chat.Provider, &chat.BaseURL, &chat.Model, &chat.APIKey,
		&chat.Temperature, &chat.ContextMessageLimit, &chat.ExtraHeaders,
		&emb.Provider, &emb.BaseURL, &emb.Model, &emb.APIKey,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ChatConfig{}, model.EmbeddingConfig{}, false, nil
		}
		return model.ChatConfig{}, model.EmbeddingConfig{}, false, fmt.Errorf("query user config: %w", err)
	}
	if s.encryptionKey != nil {
		if decrypted, err := auth.DecryptString(s.encryptionKey, chat.APIKey); err == nil {
			chat.APIKey = decrypted
		} else {
			return model.ChatConfig{}, model.EmbeddingConfig{}, false, fmt.Errorf("decrypt chat api key: %w", err)
		}
		if decrypted, err := auth.DecryptString(s.encryptionKey, emb.APIKey); err == nil {
			emb.APIKey = decrypted
		} else {
			return model.ChatConfig{}, model.EmbeddingConfig{}, false, fmt.Errorf("decrypt embedding api key: %w", err)
		}
	}
	return chat, emb, true, nil
}

// UpsertUserConfig stores or replaces the per-user chat/embedding config.
// API keys are encrypted with the store's master key before persisting.
func (s *UserStore) UpsertUserConfig(userID string, chat model.ChatConfig, emb model.EmbeddingConfig) error {
	uid := strings.TrimSpace(userID)
	if uid == "" {
		return fmt.Errorf("user id is required")
	}
	chatAPIKey := chat.APIKey
	embAPIKey := emb.APIKey
	if s.encryptionKey != nil {
		encrypted, err := auth.EncryptString(s.encryptionKey, chat.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt chat api key: %w", err)
		}
		chatAPIKey = encrypted
		encrypted, err = auth.EncryptString(s.encryptionKey, emb.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt embedding api key: %w", err)
		}
		embAPIKey = encrypted
	}
	_, err := s.db.Exec(`
		INSERT INTO user_configs (
			user_id,
			chat_provider, chat_base_url, chat_model, chat_api_key, chat_temperature, chat_context_limit, chat_extra_headers,
			embedding_provider, embedding_base_url, embedding_model, embedding_api_key,
			updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(user_id) DO UPDATE SET
			chat_provider=excluded.chat_provider,
			chat_base_url=excluded.chat_base_url,
			chat_model=excluded.chat_model,
			chat_api_key=excluded.chat_api_key,
			chat_temperature=excluded.chat_temperature,
			chat_context_limit=excluded.chat_context_limit,
			chat_extra_headers=excluded.chat_extra_headers,
			embedding_provider=excluded.embedding_provider,
			embedding_base_url=excluded.embedding_base_url,
			embedding_model=excluded.embedding_model,
			embedding_api_key=excluded.embedding_api_key,
			updated_at=excluded.updated_at`,
		uid,
		chat.Provider, chat.BaseURL, chat.Model, chatAPIKey, chat.Temperature, chat.ContextMessageLimit, chat.ExtraHeaders,
		emb.Provider, emb.BaseURL, emb.Model, embAPIKey,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert user config: %w", err)
	}
	return nil
}

// DeleteUserConfig removes a user's config row (used when a user account is deleted).
func (s *UserStore) DeleteUserConfig(userID string) error {
	uid := strings.TrimSpace(userID)
	if uid == "" {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM user_configs WHERE user_id = ?`, uid)
	if err != nil {
		return fmt.Errorf("delete user config: %w", err)
	}
	return nil
}
