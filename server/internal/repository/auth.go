package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serverwatch/serverwatch/server/internal/service"
)

var ErrUserNotFound = errors.New("user not found")

type AuthRepository struct {
	DB *pgxpool.Pool
}

// CountUsers reports how many accounts exist. It drives the one-time
// first-run administrator bootstrap: when authentication is enabled and this
// count is zero, the setup endpoint is allowed to create the first admin.
func (r *AuthRepository) CountUsers(ctx context.Context) (int, error) {
	var count int
	if err := r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// CreateUser inserts an account and returns its public representation.
func (r *AuthRepository) CreateUser(
	ctx context.Context,
	email string,
	displayName string,
	passwordHash string,
	isAdmin bool,
) (service.User, error) {
	var user service.User
	var nullableDisplayName *string
	if displayName != "" {
		nullableDisplayName = &displayName
	}

	err := r.DB.QueryRow(
		ctx,
		`
		INSERT INTO users (email, password_hash, display_name, is_admin)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, COALESCE(display_name, ''), is_admin, created_at
		`,
		email,
		passwordHash,
		nullableDisplayName,
		isAdmin,
	).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.IsAdmin,
		&user.CreatedAt,
	)
	if isUniqueViolation(err) {
		return service.User{}, ErrEmailConflict
	}
	return user, err
}

// ErrEmailConflict is returned when an email address is already registered.
var ErrEmailConflict = errors.New("email already registered")

// FindUserByEmail returns the public user plus the stored password hash. The
// hash is a separate return value so it can be verified without ever being
// embedded in the JSON-facing service.User type.
func (r *AuthRepository) FindUserByEmail(
	ctx context.Context,
	email string,
) (service.User, string, error) {
	var user service.User
	var passwordHash string

	err := r.DB.QueryRow(
		ctx,
		`
		SELECT id, email, COALESCE(display_name, ''), is_admin, created_at, password_hash
		FROM users
		WHERE email = $1 AND is_active = TRUE
		`,
		email,
	).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.IsAdmin,
		&user.CreatedAt,
		&passwordHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.User{}, "", ErrUserNotFound
	}
	if err != nil {
		return service.User{}, "", err
	}
	return user, passwordHash, nil
}

// CreateSession stores a session for a user. Only the token hash is persisted.
func (r *AuthRepository) CreateSession(
	ctx context.Context,
	tokenHash string,
	userID int64,
	expiresAt time.Time,
) error {
	_, err := r.DB.Exec(
		ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash,
		userID,
		expiresAt,
	)
	return err
}

// FindUserBySession resolves a live session token hash to its user. Expired
// sessions and deactivated accounts are treated as not found.
func (r *AuthRepository) FindUserBySession(
	ctx context.Context,
	tokenHash string,
) (service.User, error) {
	var user service.User
	err := r.DB.QueryRow(
		ctx,
		`
		SELECT u.id, u.email, COALESCE(u.display_name, ''), u.is_admin, u.created_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1
		  AND s.expires_at > NOW()
		  AND u.is_active = TRUE
		`,
		tokenHash,
	).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.IsAdmin,
		&user.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.User{}, ErrUserNotFound
	}
	if err != nil {
		return service.User{}, err
	}
	return user, nil
}

// DeleteSession removes one session (logout).
func (r *AuthRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// DeleteExpiredSessions prunes expired sessions. It is called opportunistically
// during login, matching the lightweight approach used elsewhere in the code.
func (r *AuthRepository) DeleteExpiredSessions(ctx context.Context) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= NOW()`)
	return err
}

// CountUsersWithLock creates the first admin only if no user exists yet, using
// the table itself as the concurrency guard. It returns the created user, or
// ok=false when an account already exists.
func (r *AuthRepository) CreateFirstUser(
	ctx context.Context,
	email string,
	displayName string,
	passwordHash string,
) (service.User, bool, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return service.User{}, false, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `LOCK TABLE users IN EXCLUSIVE MODE`); err != nil {
		return service.User{}, false, err
	}

	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return service.User{}, false, err
	}
	if count > 0 {
		return service.User{}, false, nil
	}

	var user service.User
	var nullableDisplayName *string
	if displayName != "" {
		nullableDisplayName = &displayName
	}
	err = tx.QueryRow(
		ctx,
		`
		INSERT INTO users (email, password_hash, display_name, is_admin)
		VALUES ($1, $2, $3, TRUE)
		RETURNING id, email, COALESCE(display_name, ''), is_admin, created_at
		`,
		email,
		passwordHash,
		nullableDisplayName,
	).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.IsAdmin,
		&user.CreatedAt,
	)
	if err != nil {
		return service.User{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return service.User{}, false, err
	}
	return user, true, nil
}
