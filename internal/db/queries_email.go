package db

import (
	"context"
	"time"
)

// EmailVerification is one email a user has requested to verify (or verified).
type EmailVerification struct {
	EmailHint string    `json:"email_hint"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
}

// UpsertEmailVerificationParams carries the fields for a verification request.
type UpsertEmailVerificationParams struct {
	UserID          string
	EmailSHA1       string
	EmailHint       string
	EmailCiphertext []byte
	EmailDEKWrapped []byte
	TokenHash       []byte
	ExpiresAt       time.Time
}

// UpsertEmailVerification stores (or refreshes) a pending verification for one
// (user, email). Re-requesting regenerates the token and clears any prior
// verified state so the user must re-confirm.
func (q *Queries) UpsertEmailVerification(ctx context.Context, p UpsertEmailVerificationParams) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO email_verifications
		   (user_id, email_sha1, email_hint, email_ciphertext, email_dek_wrapped, token_hash, expires_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (user_id, email_sha1) DO UPDATE SET
		   email_hint = EXCLUDED.email_hint,
		   email_ciphertext = EXCLUDED.email_ciphertext,
		   email_dek_wrapped = EXCLUDED.email_dek_wrapped,
		   token_hash = EXCLUDED.token_hash,
		   expires_at = EXCLUDED.expires_at,
		   created_at = NOW(),
		   verified_at = NULL`,
		p.UserID, p.EmailSHA1, p.EmailHint, p.EmailCiphertext, p.EmailDEKWrapped, p.TokenHash, p.ExpiresAt)
	return err
}

// ConfirmEmailVerification marks the email matching the token hash verified and
// returns its masked hint. ok is false if the token is unknown, already used, or
// expired.
func (q *Queries) ConfirmEmailVerification(ctx context.Context, tokenHash []byte) (hint string, ok bool) {
	err := q.pool.QueryRow(ctx,
		`UPDATE email_verifications SET verified_at = NOW()
		 WHERE token_hash = $1 AND verified_at IS NULL AND expires_at > NOW()
		 RETURNING email_hint`, tokenHash).Scan(&hint)
	if err != nil {
		return "", false
	}
	return hint, true
}

// ListEmailVerifications returns a user's emails and their verification status.
func (q *Queries) ListEmailVerifications(ctx context.Context, userID string) ([]EmailVerification, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT email_hint, verified_at IS NOT NULL, created_at
		 FROM email_verifications WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]EmailVerification, 0)
	for rows.Next() {
		var e EmailVerification
		if err := rows.Scan(&e.EmailHint, &e.Verified, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
