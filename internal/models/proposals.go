package models

import (
	"encoding/json"
	"time"
)

// Proposal statuses.
const (
	ProposalDraft     = "draft"
	ProposalPending   = "pending"
	ProposalApproved  = "approved"
	ProposalRejected  = "rejected"
	ProposalApplied   = "applied"
	ProposalWithdrawn = "withdrawn"
)

// Proposal operations.
const (
	OpCreate = "create"
	OpUpdate = "update"
	OpDelete = "delete"
)

// Entity kinds a proposal can target.
const (
	KindResource      = "resource"
	KindResourceGroup = "resource_group"
	KindSite          = "site"
	KindFacility      = "facility"
	KindProject       = "project"
	KindDowntime      = "downtime"
)

// Proposal is a living draft of a create/update/delete change to a topology
// entity. proposed_state is the mutable head; the full edit history lives in
// change_proposal_revisions.
type Proposal struct {
	ID               string             `json:"id"`
	EntityKind       string             `json:"entity_kind"`
	TargetName       string             `json:"target_name,omitempty"`
	Operation        string             `json:"operation"`
	ProposedState    json.RawMessage    `json:"proposed_state"`
	SchemaVersion    int                `json:"schema_version"`
	BaseVersion      json.RawMessage    `json:"base_version,omitempty"`
	Status           string             `json:"status"`
	CreatedBy        string             `json:"created_by"`
	AssignedReviewer string             `json:"assigned_reviewer,omitempty"`
	ReviewNote       string             `json:"review_note,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	Revisions        []ProposalRevision `json:"revisions,omitempty"`
}

// ProposalRevision is one entry in a proposal's append-only edit history.
type ProposalRevision struct {
	RevisionNo    int             `json:"revision_no"`
	ProposedState json.RawMessage `json:"proposed_state"`
	EditedBy      string          `json:"edited_by"`
	Note          string          `json:"note,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// AuditEntry is one immutable audit-log row.
type AuditEntry struct {
	ID          string          `json:"id"`
	ActorUserID string          `json:"actor_user_id,omitempty"`
	Action      string          `json:"action"`
	EntityKind  string          `json:"entity_kind,omitempty"`
	EntityID    string          `json:"entity_id,omitempty"`
	ProposalID  string          `json:"proposal_id,omitempty"`
	Detail      json.RawMessage `json:"detail,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}
