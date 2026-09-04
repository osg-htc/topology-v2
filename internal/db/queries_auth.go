package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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
	var legacy, username *string
	err := q.pool.QueryRow(ctx,
		`SELECT id, display_name, username, status, legacy_contact_id, is_provisioned,
		        last_login, created_at, updated_at
		 FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.DisplayName, &username, &u.Status, &legacy, &u.IsProvisioned,
			&u.LastLogin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.LegacyContactID = deref(legacy)
	u.Username = deref(username)
	return u, nil
}

// EnsureUsername sets a unique username on a user if it has none, deriving it
// from base (e.g. the OIDC preferred_username) with a numeric suffix on
// collision. Returns the user's username.
func (q *Queries) EnsureUsername(ctx context.Context, userID, base string) (string, error) {
	var existing *string
	if err := q.pool.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, userID).Scan(&existing); err != nil {
		return "", err
	}
	if existing != nil && *existing != "" {
		return *existing, nil
	}
	clean := SanitizeUsername(base)
	if clean == "" {
		clean = "user"
	}
	for i := 0; i < 200; i++ {
		cand := clean
		if i > 0 {
			cand = fmt.Sprintf("%s-%d", clean, i+1)
		}
		tag, err := q.pool.Exec(ctx,
			`UPDATE users SET username = $2, updated_at = NOW()
			 WHERE id = $1 AND NOT EXISTS (SELECT 1 FROM users WHERE username = $2)`,
			userID, cand)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() == 1 {
			return cand, nil
		}
	}
	return "", errors.New("could not allocate a unique username")
}

// SetUsername lets an administrator change a user's username (must be unique).
func (q *Queries) SetUsername(ctx context.Context, userID, username string) error {
	clean := SanitizeUsername(username)
	if clean == "" {
		return errors.New("invalid username")
	}
	tag, err := q.pool.Exec(ctx,
		`UPDATE users SET username = $2, updated_at = NOW()
		 WHERE id = $1 AND NOT EXISTS (SELECT 1 FROM users WHERE username = $2 AND id <> $1)`,
		userID, clean)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("username already taken")
	}
	return nil
}

// SanitizeUsername lowercases and strips a base string to [a-z0-9._-].
func SanitizeUsername(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Prefer the local part if an email was passed.
	if at := strings.IndexByte(s, '@'); at > 0 {
		s = s[:at]
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), ".-_")
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

// UpsertProvisionedContactUser ensures a (provisioned, identity-less) user
// exists for a legacy contact id, returning its id. Contacts must be users;
// this bootstraps an account for an OSG contact without linking any identity.
// Returns "" if legacyID is empty (cannot dedupe an id-less contact).
func (q *Queries) UpsertProvisionedContactUser(ctx context.Context, name, legacyID string) (string, error) {
	if legacyID == "" {
		return "", nil
	}
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO users (display_name, status, is_provisioned, legacy_contact_id)
		 VALUES ($1, 'active', TRUE, $2)
		 ON CONFLICT (legacy_contact_id) WHERE legacy_contact_id IS NOT NULL
		 DO UPDATE SET display_name = CASE WHEN users.display_name = '' THEN EXCLUDED.display_name ELSE users.display_name END
		 RETURNING id`, name, legacyID).Scan(&id)
	return id, err
}

// BackfillContactUsers creates provisioned users for every distinct legacy
// contact id in resource_contacts and links resource_contacts.user_id. Idempotent.
func (q *Queries) BackfillContactUsers(ctx context.Context) (int, error) {
	if _, err := q.pool.Exec(ctx,
		`INSERT INTO users (display_name, status, is_provisioned, legacy_contact_id)
		 SELECT DISTINCT ON (contact_id) COALESCE(contact_name,''), 'active', TRUE, contact_id
		 FROM resource_contacts
		 WHERE COALESCE(contact_id,'') <> ''
		   AND NOT EXISTS (SELECT 1 FROM users u WHERE u.legacy_contact_id = resource_contacts.contact_id)
		 ORDER BY contact_id, contact_name
		 ON CONFLICT (legacy_contact_id) WHERE legacy_contact_id IS NOT NULL DO NOTHING`); err != nil {
		return 0, err
	}
	tag, err := q.pool.Exec(ctx,
		`UPDATE resource_contacts rc SET user_id = u.id
		 FROM users u
		 WHERE u.legacy_contact_id = rc.contact_id
		   AND rc.user_id IS NULL AND COALESCE(rc.contact_id,'') <> ''`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// BackfillEntityContactUsers is BackfillContactUsers' twin for
// entity_contacts (resource_group/site/facility contacts) -- kind-agnostic,
// since the table is already shared across all three.
func (q *Queries) BackfillEntityContactUsers(ctx context.Context) (int, error) {
	if _, err := q.pool.Exec(ctx,
		`INSERT INTO users (display_name, status, is_provisioned, legacy_contact_id)
		 SELECT DISTINCT ON (contact_id) COALESCE(contact_name,''), 'active', TRUE, contact_id
		 FROM entity_contacts
		 WHERE COALESCE(contact_id,'') <> ''
		   AND NOT EXISTS (SELECT 1 FROM users u WHERE u.legacy_contact_id = entity_contacts.contact_id)
		 ORDER BY contact_id, contact_name
		 ON CONFLICT (legacy_contact_id) WHERE legacy_contact_id IS NOT NULL DO NOTHING`); err != nil {
		return 0, err
	}
	tag, err := q.pool.Exec(ctx,
		`UPDATE entity_contacts ec SET user_id = u.id
		 FROM users u
		 WHERE u.legacy_contact_id = ec.contact_id
		   AND ec.user_id IS NULL AND COALESCE(ec.contact_id,'') <> ''`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// UserExists reports whether id refers to a real users row. Casts the
// column to text before comparing so a garbage, non-UUID id (e.g. from a
// crafted request) reports false instead of raising a raw Postgres type
// error -- the same class of bug already hit once this session (the
// bundle-approve UUID crash) when an untrusted string reached a uuid column
// directly.
// LegacyContactIDExists reports whether id matches a real user's
// legacy_contact_id -- the one identifier a contact has (see EntityContact's
// doc comment in queries_contacts.go). This is the sole check
// requireResolvedContacts/requireResolvedResourceContacts rely on.
func (q *Queries) LegacyContactIDExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := q.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE legacy_contact_id = $1)`, id).Scan(&ok)
	return ok, err
}

// SetLegacyContactIDIfMissing mints this user's canonical contact id (the v1
// SHA1-of-email scheme, via emailSHA1) the moment we learn their email --
// login or invite-onboarding -- if they don't already have one. A unique
// violation (another user somehow already claims this id) is swallowed
// rather than erroring the caller's login/onboarding flow over it.
func (q *Queries) SetLegacyContactIDIfMissing(ctx context.Context, userID, legacyID string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE users SET legacy_contact_id = $2 WHERE id = $1 AND legacy_contact_id IS NULL`,
		userID, legacyID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil
	}
	return err
}

// SearchUsers finds users by display name or legacy contact id (admin picker).
func (q *Queries) SearchUsers(ctx context.Context, query string, limit int) ([]*models.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := q.pool.Query(ctx,
		`SELECT id, display_name, username, status, legacy_contact_id, is_provisioned,
		        last_login, created_at, updated_at
		 FROM users
		 WHERE display_name ILIKE '%'||$1||'%' OR legacy_contact_id ILIKE '%'||$1||'%'
		    OR username ILIKE '%'||$1||'%'
		 ORDER BY display_name LIMIT $2`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.User
	for rows.Next() {
		u := &models.User{}
		var legacy, username *string
		if err := rows.Scan(&u.ID, &u.DisplayName, &username, &u.Status, &legacy, &u.IsProvisioned,
			&u.LastLogin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.LegacyContactID = deref(legacy)
		u.Username = deref(username)
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListAllUsers returns all user accounts, newest first (admin user management).
func (q *Queries) ListAllUsers(ctx context.Context) ([]*models.User, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, display_name, username, status, legacy_contact_id, is_provisioned,
		        last_login, created_at, updated_at
		 FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.User
	for rows.Next() {
		u := &models.User{}
		var legacy, username *string
		if err := rows.Scan(&u.ID, &u.DisplayName, &username, &u.Status, &legacy, &u.IsProvisioned,
			&u.LastLogin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.LegacyContactID = deref(legacy)
		u.Username = deref(username)
		out = append(out, u)
	}
	return out, rows.Err()
}

// UserLabel is the lightweight actor-display for "Display name (username)".
type UserLabel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
}

// UserLabels returns display labels for a set of user ids (for showing actors).
func (q *Queries) UserLabels(ctx context.Context, ids []string) ([]UserLabel, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, display_name, COALESCE(username,'') FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]UserLabel, 0, len(ids))
	for rows.Next() {
		var l UserLabel
		if err := rows.Scan(&l.ID, &l.DisplayName, &l.Username); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
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
	// ContactEmail is the email typed while onboarding a brand-new contact
	// (contact_onboard invites only) -- stored so it's not silently
	// discarded; this app has no mailer, so nothing sends to it.
	ContactEmail string
}

// CreateInvite inserts an invite and returns its id.
func (q *Queries) CreateInvite(ctx context.Context, p CreateInviteParams) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO invites (kind, token_hash, created_by, target_user_id, claim, expires_at, contact_email)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		p.Kind, p.TokenHash, nullString(p.CreatedBy), nullString(p.TargetUserID),
		nullBytes(p.ClaimJSON), p.ExpiresAt, nullString(p.ContactEmail)).Scan(&id)
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
	ContactEmail string
}

// GetInviteByTokenHash fetches an invite by its token hash.
func (q *Queries) GetInviteByTokenHash(ctx context.Context, tokenHash []byte) (*InviteRow, error) {
	r := &InviteRow{}
	var createdBy, target, contactEmail *string
	err := q.pool.QueryRow(ctx,
		`SELECT id, kind, created_by, target_user_id, claim, used_at, expires_at, created_at, contact_email
		 FROM invites WHERE token_hash = $1`, tokenHash).
		Scan(&r.ID, &r.Kind, &createdBy, &target, &r.ClaimJSON, &r.UsedAt, &r.ExpiresAt, &r.CreatedAt, &contactEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.CreatedBy = deref(createdBy)
	r.TargetUserID = deref(target)
	r.ContactEmail = deref(contactEmail)
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
