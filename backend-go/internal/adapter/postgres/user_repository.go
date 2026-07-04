package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/domain/user"
)

const userColumns = `
	id,
	username,
	email,
	hashed_password,
	role::text,
	display_name,
	avatar_url,
	is_active,
	status::text,
	last_login_at,
	created_at,
	updated_at`

// UserRepository persists users, auth settings, and public password reset requests.
type UserRepository struct {
	Repository
	beginner pgxTxBeginner
}

// NewUserRepository creates a PostgreSQL-backed user repository.
func NewUserRepository(db Querier) (UserRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return UserRepository{}, err
	}
	repo := UserRepository{Repository: base}
	if beginner, ok := db.(pgxTxBeginner); ok {
		repo.beginner = beginner
	}
	return repo, nil
}

// GetByUsername returns a user by username.
func (r UserRepository) GetByUsername(ctx context.Context, username string) (user.User, bool, error) {
	row := r.DB().QueryRow(ctx, `SELECT `+userColumns+` FROM public.users WHERE username = $1`, username)
	return scanOptionalUser(row)
}

// GetByEmail returns a user by email.
func (r UserRepository) GetByEmail(ctx context.Context, email string) (user.User, bool, error) {
	row := r.DB().QueryRow(ctx, `SELECT `+userColumns+` FROM public.users WHERE email = $1`, email)
	return scanOptionalUser(row)
}

// GetByID returns a user by ID.
func (r UserRepository) GetByID(ctx context.Context, id string) (user.User, bool, error) {
	row := r.DB().QueryRow(ctx, `SELECT `+userColumns+` FROM public.users WHERE id = $1`, id)
	return scanOptionalUser(row)
}

// Create inserts a new user and returns the persisted row.
func (r UserRepository) Create(ctx context.Context, input user.CreateUser) (user.User, error) {
	if input.ID == "" {
		id, err := newUUID()
		if err != nil {
			return user.User{}, err
		}
		input.ID = id
	}
	if input.Status == "" {
		input.Status = user.StatusActive
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = input.CreatedAt
	}

	row := r.DB().QueryRow(ctx, `
		INSERT INTO public.users (
			id,
			username,
			email,
			hashed_password,
			role,
			display_name,
			avatar_url,
			is_active,
			status,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5::public.userrole, $6, NULL, $7, $8::public.userstatus, $9, $10)
		RETURNING `+userColumns,
		input.ID,
		input.Username,
		input.Email,
		input.HashedPassword,
		input.Role.DBValue(),
		input.DisplayName,
		input.IsActive,
		input.Status.DBValue(),
		input.CreatedAt,
		input.UpdatedAt,
	)
	account, ok, err := scanOptionalUser(row)
	if err != nil {
		return user.User{}, err
	}
	if !ok {
		return user.User{}, pgx.ErrNoRows
	}
	return account, nil
}

// UpdatePassword updates the password hash and timestamp for one user.
func (r UserRepository) UpdatePassword(ctx context.Context, userID string, hashedPassword string) error {
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.users
		SET hashed_password = $2, updated_at = $3
		WHERE id = $1`,
		userID,
		hashedPassword,
		time.Now().UTC(),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// RegistrationSettings reads the public registration toggles with Python-compatible defaults.
func (r UserRepository) RegistrationSettings(ctx context.Context) (authapp.RegistrationSettings, error) {
	settings := authapp.RegistrationSettings{AllowStudent: true, AllowTeacher: false}
	rows, err := r.DB().Query(ctx, `
		SELECT key, value
		FROM public.system_settings
		WHERE key IN ('allow_student_registration', 'allow_teacher_registration')`)
	if err != nil {
		return settings, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return settings, err
		}
		switch key {
		case "allow_student_registration":
			settings.AllowStudent = strings.EqualFold(value, "true")
		case "allow_teacher_registration":
			settings.AllowTeacher = strings.EqualFold(value, "true")
		}
	}
	return settings, rows.Err()
}

// CountPasswordResetRequestsSince counts reset requests for a user in a time window.
func (r UserRepository) CountPasswordResetRequestsSince(ctx context.Context, userID string, since time.Time) (int, error) {
	var count int
	if err := r.DB().QueryRow(ctx, `
		SELECT count(*)
		FROM public.password_reset_requests
		WHERE user_id = $1 AND created_at >= $2`,
		userID,
		since,
	).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// HasPendingPasswordReset reports whether a user already has a pending request.
func (r UserRepository) HasPendingPasswordReset(ctx context.Context, userID string) (bool, error) {
	return r.Exists(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM public.password_reset_requests
			WHERE user_id = $1 AND status = 'pending'::public.passwordresetstatus
		)`,
		userID,
	)
}

// CreatePasswordResetRequest inserts a new public password reset request.
func (r UserRepository) CreatePasswordResetRequest(ctx context.Context, request authapp.PasswordResetRequest) (string, error) {
	id, err := newUUID()
	if err != nil {
		return "", err
	}
	if request.CreatedAt.IsZero() {
		request.CreatedAt = time.Now().UTC()
	}
	_, err = r.DB().Exec(ctx, `
		INSERT INTO public.password_reset_requests (
			id,
			user_id,
			username,
			email,
			reason,
			status,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, 'pending'::public.passwordresetstatus, $6)`,
		id,
		request.UserID,
		request.Username,
		request.Email,
		request.Reason,
		request.CreatedAt,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

// LatestPasswordResetRequestStatus returns the latest reset request status for a matching account.
func (r UserRepository) LatestPasswordResetRequestStatus(ctx context.Context, username, email string) (authapp.PasswordResetStatus, bool, error) {
	var status string
	var createdAt time.Time
	err := r.DB().QueryRow(ctx, `
		SELECT pr.status::text, pr.created_at
		FROM public.password_reset_requests pr
		JOIN public.users u ON u.id = pr.user_id
		WHERE u.username = $1 AND u.email = $2
		ORDER BY pr.created_at DESC
		LIMIT 1`,
		username,
		email,
	).Scan(&status, &createdAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return authapp.PasswordResetStatus{}, false, nil
		}
		return authapp.PasswordResetStatus{}, false, err
	}
	return authapp.PasswordResetStatus{
		HasPending: status == "pending",
		Status:     &status,
		CreatedAt:  &createdAt,
	}, true, nil
}

func scanOptionalUser(row rowScanner) (user.User, bool, error) {
	var account user.User
	var roleValue string
	var statusValue string
	var displayName pgtype.Text
	var avatarURL pgtype.Text
	var lastLoginAt pgtype.Timestamp

	err := row.Scan(
		&account.ID,
		&account.Username,
		&account.Email,
		&account.HashedPassword,
		&roleValue,
		&displayName,
		&avatarURL,
		&account.IsActive,
		&statusValue,
		&lastLoginAt,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return user.User{}, false, nil
		}
		return user.User{}, false, err
	}

	role, err := user.ParseRole(roleValue)
	if err != nil {
		return user.User{}, false, err
	}
	status, err := user.ParseStatus(statusValue)
	if err != nil {
		return user.User{}, false, err
	}
	account.Role = role
	account.Status = status
	account.DisplayName = textPtr(displayName)
	account.AvatarURL = textPtr(avatarURL)
	if lastLoginAt.Valid {
		value := lastLoginAt.Time
		account.LastLoginAt = &value
	}
	return account, true, nil
}
