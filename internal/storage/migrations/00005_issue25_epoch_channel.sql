-- +goose Up
-- All v2 channels are opaque membership-epoch UUIDs (not account/conversation IDs).
-- The server authorizes subscriptions against active membership for the epoch.
ALTER TABLE issue25_publish_attempts
 ADD CONSTRAINT issue25_epoch_channel_match CHECK (
  opaque_channel = 'chat:e:' || membership_epoch::text
 );
-- +goose Down
ALTER TABLE issue25_publish_attempts DROP CONSTRAINT issue25_epoch_channel_match;
