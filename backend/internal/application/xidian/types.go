package xidian

import "time"

// Cookie is a serializable HTTP cookie used only inside a short-lived login challenge.
type Cookie map[string]any

// Account is a bound Xidian account.
type Account struct {
	ID             string
	UserID         string
	Username       string
	Status         string
	LastVerifiedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AccountUpsert stores a successful account binding.
type AccountUpsert struct {
	ID             string
	UserID         string
	Username       string
	LastVerifiedAt time.Time
	Now            time.Time
}

// Challenge contains public captcha fields plus private login state.
type Challenge struct {
	CaptchaBig   string
	CaptchaPiece string
	PieceY       int
	State        ChallengeState
}

// ChallengeState is the private state needed to complete IDS login.
type ChallengeState struct {
	ServiceURL   string
	HiddenInputs map[string]string
	PasswordSalt string
	Cookies      []Cookie
	CreatedAt    time.Time
	Raw          map[string]any
}

// LoginInput completes one captcha login.
type LoginInput struct {
	Username       string
	Password       string
	SliderPosition float64
}

// BindingStatus is returned by GET /xidian/binding.
type BindingStatus struct {
	IsBound        bool       `json:"is_bound"`
	Username       *string    `json:"username,omitempty"`
	LastVerifiedAt *time.Time `json:"last_verified_at,omitempty"`
}

// BindStartResponse is returned by POST /xidian/binding/start.
type BindStartResponse struct {
	ChallengeID  string `json:"challenge_id"`
	CaptchaBig   string `json:"captcha_big"`
	CaptchaPiece string `json:"captcha_piece"`
	PuzzleWidth  int    `json:"puzzle_width"`
	PuzzleHeight int    `json:"puzzle_height"`
	PieceWidth   int    `json:"piece_width"`
	PieceHeight  int    `json:"piece_height"`
	PieceY       int    `json:"piece_y"`
}

// CompleteBindingInput is parsed from POST /xidian/binding/complete.
type CompleteBindingInput struct {
	ChallengeID    string  `json:"challenge_id"`
	SliderPosition float64 `json:"slider_position"`
	Username       *string `json:"username"`
	Password       *string `json:"password"`
}

// BindCompleteResponse is returned after successful binding.
type BindCompleteResponse struct {
	IsBound        bool       `json:"is_bound"`
	Username       string     `json:"username"`
	LastVerifiedAt *time.Time `json:"last_verified_at,omitempty"`
}

// UnbindResponse is returned by POST /xidian/binding/unbind.
type UnbindResponse struct {
	Success bool `json:"success"`
}

// ServiceError carries Python-compatible Xidian error details.
type ServiceError struct {
	Code    string
	Message string
	Status  int
	Err     error
}
