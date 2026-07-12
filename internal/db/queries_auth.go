package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bbockelm/topology-v2/internal/models"
)

// ErrNotFound is returned when a lookup yields no rows.
var ErrNotFound = errors.New("not found")

// ---- Users ----

// CreateUserParams holds fields for creating a user (or provisioned contact).
type CreateUserParams struct {
	DisplayName     string
	Status          string
	LegacyContactID string
	IsProvisioned   bool
}

// CreateUser inserts a user and returns its id.
func (q *Queries) CreateUser(ctx context.Context, p CreateUserParams) (string, error) {
	status := p.Status
	if status == "" {
		status = "active"
	}
	var legacy *string
	if p.LegacyContactID != "" {
		legacy = &p.LegacyContactID
	}
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO users (display_name, status, legacy_contact_id, is_provisioned)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		p.DisplayName, status, legacy, p.IsProvisioned).Scan(&id)
	return id, err
}

// GetUser fetches a user by id.
func (q *Queries) GetUser(ctx context.Context, id string) (*models.User, error) {
	u := &models.User{}
	var legacy *string
	err := q.pool.QueryRow(ctx,
		`SELECT id, display_name, status, legacy_contact_id, is_provisioned,
		        last_login, created_at, updated_at
		 FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.DisplayName, &u.Status, &legacy, &u.IsProvisioned,
			&u.LastLogin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if legacy != nil {
		u.LegacyContactID = *legacy
	}
	return u, nil
}

// UpdateUserLastLogin stamps last_login and marks the account claimed.
func (q *Queries) UpdateUserLastLogin(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE users SET last_login = NOW(), is_provisioned = FALSE, updated_at = NOW()
		 WHERE id = $1`, id)
	return err
}

// UpdateUserDisplayName sets the display name.
func (q *Queries) UpdateUserDisplayName(ctx context.Context, id, name string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE users SET display_name = $2, updated_at = NOW() WHERE id = $1`, id, name)
	return err
}

// FindUserByLegacyContactID looks up a provisioned contact by its legacy id.
func (q *Queries) FindUserByLegacyContactID(ctx context.Context, legacyID string) (*models.User, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE legacy_contact_id = $1`, legacyID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return q.GetUser(ctx, id)
}

// ---- Roles ----

// GetUserRoles returns the roles assigned to a user.
func (q *Queries) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := q.pool.Query(ctx, `SELECT role FROM user_roles WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// AddUserRole grants a role (idempotent).
func (q *Queries) AddUserRole(ctx context.Context, userID, role string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO user_roles (user_id, role) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, role)
	return err
}

// RemoveUserRole revokes a role.
func (q *Queries) RemoveUserRole(ctx context.Context, userID, role string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1 AND role = $2`, userID, role)
	return err
}

// ---- Identities ----

// CreateIdentityParams holds the fields for linking a federated identity,
// including the pre-encrypted email columns.
type CreateIdentityParams struct {
	UserID          string
	Issuer          string
	Subject         string
	EmailCiphertext []byte
	EmailDEKWrapped []byte
	EmailSHA1       string
	EPPN            string
	OIDC            string
	CILogonID       string
	IdPName         string
	DisplayName     string
}

// CreateIdentity inserts a user_identity. A pgx unique-violation (23505) on
// (issuer, subject) signals the identity is already linked to another account.
func (q *Queries) CreateIdentity(ctx context.Context, p CreateIdentityParams) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO user_identities
		   (user_id, issuer, subject, email_ciphertext, email_dek_wrapped, email_sha1,
		    eppn, oidc, cilogon_id, idp_name, display_name)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		p.UserID, p.Issuer, p.Subject, p.EmailCiphertext, p.EmailDEKWrapped,
		nullString(p.EmailSHA1), nullString(p.EPPN), nullString(p.OIDC),
		nullString(p.CILogonID), nullString(p.IdPName), nullString(p.DisplayName)).
		Scan(&id)
	return id, err
}

// identityRow scans a full identity including encrypted email columns.
type identityRow struct {
	models.UserIdentity
	EmailCiphertext []byte
	EmailDEKWrapped []byte
}

// FindIdentity looks up an identity by (issuer, subject).
func (q *Queries) FindIdentity(ctx context.Context, issuer, subject string) (*identityRow, error) {
	return q.scanIdentity(ctx,
		`SELECT id, user_id, issuer, subject, email_ciphertext, email_dek_wrapped,
		        email_sha1, eppn, oidc, cilogon_id, idp_name, display_name, created_at
		 FROM user_identities WHERE issuer = $1 AND subject = $2`, issuer, subject)
}

func (q *Queries) scanIdentity(ctx context.Context, sql string, args ...any) (*identityRow, error) {
	r := &identityRow{}
	var emailSHA1, eppn, oidc, cilogon, idp, display *string
	err := q.pool.QueryRow(ctx, sql, args...).Scan(
		&r.ID, &r.UserID, &r.Issuer, &r.Subject, &r.EmailCiphertext, &r.EmailDEKWrapped,
		&emailSHA1, &eppn, &oidc, &cilogon, &idp, &display, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.EmailSHA1 = deref(emailSHA1)
	r.EPPN = deref(eppn)
	r.OIDC = deref(oidc)
	r.CILogonID = deref(cilogon)
	r.IdPName = deref(idp)
	r.DisplayName = deref(display)
	return r, nil
}

// ListUserIdentities returns all identities for a user (encrypted email columns
// exposed via identityRow for the caller to decrypt if authorized).
func (q *Queries) ListUserIdentities(ctx context.Context, userID string) ([]*identityRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, user_id, issuer, subject, email_ciphertext, email_dek_wrapped,
		        email_sha1, eppn, oidc, cilogon_id, idp_name, display_name, created_at
		 FROM user_identities WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*identityRow
	for rows.Next() {
		r := &identityRow{}
		var emailSHA1, eppn, oidc, cilogon, idp, display *string
		if err := rows.Scan(&r.ID, &r.UserID, &r.Issuer, &r.Subject,
			&r.EmailCiphertext, &r.EmailDEKWrapped, &emailSHA1, &eppn, &oidc,
			&cilogon, &idp, &display, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.EmailSHA1 = deref(emailSHA1)
		r.EPPN = deref(eppn)
		r.OIDC = deref(oidc)
		r.CILogonID = deref(cilogon)
		r.IdPName = deref(idp)
		r.DisplayName = deref(display)
		out = append(out, r)
	}
	return out, rows.Err()
}

// EncryptedEmailByContactID resolves a legacy contact id (OSG/CILogon id or
// SHA1-of-email) to the encrypted email of a matching identity, if any. Used to
// expose contact emails to authorized API clients.
func (q *Queries) EncryptedEmailByContactID(ctx context.Context, contactID string) (ciphertext, wrapped []byte, ok bool) {
	err := q.pool.QueryRow(ctx,
		`SELECT email_ciphertext, email_dek_wrapped
		 FROM user_identities
		 WHERE (email_sha1 = $1 OR cilogon_id = $1) AND email_ciphertext IS NOT NULL
		 LIMIT 1`, contactID).Scan(&ciphertext, &wrapped)
	if err != nil {
		return nil, nil, false
	}
	return ciphertext, wrapped, true
}

// DeleteIdentity unlinks a federated identity from an account.
func (q *Queries) DeleteIdentity(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM user_identities WHERE id = $1`, id)
	return err
}

// EmailCiphertext / EmailDEKWrapped accessors for the identity row.
func (r *identityRow) Encrypted() ([]byte, []byte) { return r.EmailCiphertext, r.EmailDEKWrapped }

// Identity returns the embedded public model.
func (r *identityRow) Identity() models.UserIdentity { return r.UserIdentity }

// ---- Sessions ----

// CreateSession inserts a session row keyed by the token hash.
func (q *Queries) CreateSession(ctx context.Context, userID, role string, tokenHash []byte, expiresAt time.Time) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO sessions (user_id, role, token_hash, expires_at)
		 VALUES ($1,$2,$3,$4) RETURNING id`,
		userID, role, tokenHash, expiresAt).Scan(&id)
	return id, err
}

// GetSessionByTokenHash returns a non-expired session by token hash.
func (q *Queries) GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (*models.Session, error) {
	s := &models.Session{}
	err := q.pool.QueryRow(ctx,
		`SELECT id, user_id, role, expires_at, created_at
		 FROM sessions WHERE token_hash = $1 AND expires_at > NOW()`, tokenHash).
		Scan(&s.ID, &s.UserID, &s.Role, &s.ExpiresAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// DeleteSession removes a single session by token hash.
func (q *Queries) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// DeleteUserSessions removes all sessions for a user.
func (q *Queries) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

// ---- Invites ----

// CreateInviteParams holds fields for creating an invite.
type CreateInviteParams struct {
	Kind         string
	TokenHash    []byte
	CreatedBy    string
	TargetUserID string
	ClaimJSON    []byte // marshaled RoleClaim, or nil
	ExpiresAt    time.Time
}

// CreateInvite inserts an invite and returns its id.
func (q *Queries) CreateInvite(ctx context.Context, p CreateInviteParams) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO invites (kind, token_hash, created_by, target_user_id, claim, expires_at)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		p.Kind, p.TokenHash, nullString(p.CreatedBy), nullString(p.TargetUserID),
		nullBytes(p.ClaimJSON), p.ExpiresAt).Scan(&id)
	return id, err
}

// InviteRow is an invite plus its raw claim JSON.
type InviteRow struct {
	ID           string
	Kind         string
	CreatedBy    string
	TargetUserID string
	ClaimJSON    []byte
	UsedAt       *time.Time
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

// GetInviteByTokenHash fetches an invite by its token hash.
func (q *Queries) GetInviteByTokenHash(ctx context.Context, tokenHash []byte) (*InviteRow, error) {
	r := &InviteRow{}
	var createdBy, target *string
	err := q.pool.QueryRow(ctx,
		`SELECT id, kind, created_by, target_user_id, claim, used_at, expires_at, created_at
		 FROM invites WHERE token_hash = $1`, tokenHash).
		Scan(&r.ID, &r.Kind, &createdBy, &target, &r.ClaimJSON, &r.UsedAt, &r.ExpiresAt, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.CreatedBy = deref(createdBy)
	r.TargetUserID = deref(target)
	return r, nil
}

// MarkInviteUsed marks an invite consumed by a user.
func (q *Queries) MarkInviteUsed(ctx context.Context, id, usedBy string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE invites SET used_at = NOW(), used_by = $2 WHERE id = $1`, id, nullString(usedBy))
	return err
}

// ---- app_config ----

// GetConfig returns an app_config value, or "" if unset.
func (q *Queries) GetConfig(ctx context.Context, key string) (string, error) {
	var v string
	err := q.pool.QueryRow(ctx, `SELECT value FROM app_config WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// SetConfig upserts an app_config value.
func (q *Queries) SetConfig(ctx context.Context, key, value string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO app_config (key, value, updated_at) VALUES ($1,$2,NOW())
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`, key, value)
	return err
}

// ---- helpers ----

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
