-- +goose Up
-- Stores the email typed while onboarding a brand-new contact via invite --
-- previously decoded from the request and silently discarded. No mailer
-- exists in this app; this only persists what's already typed so it's
-- available (e.g. shown on the invite/accept page), not sent anywhere.
ALTER TABLE invites ADD COLUMN contact_email TEXT;

-- +goose Down
ALTER TABLE invites DROP COLUMN contact_email;
