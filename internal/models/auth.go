// Package models holds plain data structs (DB rows and API DTOs) with json tags.
package models

import "time"

// Role constants. A user may hold several; a session carries the highest
// privilege level, but contact_reader is an orthogonal *capability* (see
// HasContactReader) rather than a level.
const (
	RoleAdministrator = "administrator"
	RoleManager       = "manager"
	RoleUser          = "user"
	// RoleContactReader may view contact PII (emails, ids). Without it, only
	// contact names are visible.
	RoleContactReader = "contact_reader"
)

// roleRank orders the privilege-level roles from most to least privileged.
// contact_reader is intentionally absent — it is a capability, not a level.
var roleRank = map[string]int{
	RoleAdministrator: 3,
	RoleManager:       2,
	RoleUser:          1,
}

// HasContactReader reports whether the role set may view contact PII. Managers
// and administrators always can; others need the contact_reader role.
func HasContactReader(roles []string) bool {
	for _, r := range roles {
		if r == RoleContactReader || r == RoleManager || r == RoleAdministrator {
			return true
		}
	}
	return false
}

// EffectiveRole returns the highest-privilege role in the set, or RoleUser.
func EffectiveRole(roles []string) string {
	best := RoleUser
	bestRank := 0
	for _, r := range roles {
		if roleRank[r] > bestRank {
			bestRank = roleRank[r]
			best = r
		}
	}
	return best
}

// User is an account. Contacts imported from the legacy topology data are
// provisioned users (IsProvisioned=true) until someone logs in and claims them.
type User struct {
	ID              string     `json:"id"`
	DisplayName     string     `json:"display_name"`
	Username        string     `json:"username,omitempty"`
	Status          string     `json:"status"`
	LegacyContactID string     `json:"legacy_contact_id,omitempty"`
	IsProvisioned   bool       `json:"is_provisioned"`
	LastLogin       *time.Time `json:"last_login,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// Populated by joins where relevant.
	Roles      []string       `json:"roles,omitempty"`
	Identities []UserIdentity `json:"identities,omitempty"`
}

// UserIdentity is one federated identity linked to a User. Email is decrypted
// only for authorized viewers; the encrypted columns never serialize.
type UserIdentity struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Issuer      string    `json:"issuer"`
	Subject     string    `json:"subject"`
	Email       string    `json:"email,omitempty"` // decrypted on demand
	EmailSHA1   string    `json:"email_sha1,omitempty"`
	EPPN        string    `json:"eppn,omitempty"`
	OIDC        string    `json:"oidc,omitempty"`
	CILogonID   string    `json:"cilogon_id,omitempty"`
	IdPName     string    `json:"idp_name,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Session is a server-side session record (the raw token lives only in the
// client cookie; we store its SHA-256 hash).
type Session struct {
	ID        string
	UserID    string
	Role      string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Invite is a single-use invitation, either to link a federated identity to an
// existing account or to claim a responsibility (role_claim).
type Invite struct {
	ID           string     `json:"id"`
	Kind         string     `json:"kind"`
	CreatedBy    string     `json:"created_by,omitempty"`
	TargetUserID string     `json:"target_user_id,omitempty"`
	Claim        *RoleClaim `json:"claim,omitempty"`
	UsedAt       *time.Time `json:"used_at,omitempty"`
	UsedBy       string     `json:"used_by,omitempty"`
	ExpiresAt    time.Time  `json:"expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Invite kinds.
const (
	InviteAccountLink        = "account_link"
	InviteRoleClaim          = "role_claim"
	InviteReplacementRequest = "replacement_request"
	InviteContactOnboard     = "contact_onboard"
)

// RoleClaim describes a responsibility offered by a role_claim invite, e.g.
// becoming the Security Contact (rank Primary) on a given resource.
type RoleClaim struct {
	EntityKind  string `json:"entity_kind"` // resource | resource_group | ...
	EntityID    string `json:"entity_id"`
	ContactType string `json:"contact_type"` // Security Contact | Administrative Contact | ...
	Rank        string `json:"rank"`         // Primary | Secondary | Tertiary
}

// SessionInfo is the /auth/me DTO.
type SessionInfo struct {
	User          User     `json:"user"`
	EffectiveRole string   `json:"effective_role"`
	Roles         []string `json:"roles"`
}
