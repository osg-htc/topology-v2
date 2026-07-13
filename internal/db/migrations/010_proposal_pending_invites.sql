-- +goose Up
-- Links a change proposal to the onboarding invites for any brand-new contacts
-- it introduces. A proposal cannot be approved while any linked invite is still
-- unaccepted, so a change never commits with an unresolved "invited" contact.
CREATE TABLE proposal_pending_invites (
    proposal_id UUID NOT NULL REFERENCES change_proposals(id) ON DELETE CASCADE,
    invite_id   UUID NOT NULL REFERENCES invites(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (proposal_id, invite_id)
);

-- +goose Down
DROP TABLE proposal_pending_invites;
