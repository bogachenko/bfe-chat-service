-- +goose Up
-- Issue 25 additive schema: preserve all legacy owner-scoped objects.
CREATE TABLE issue25_conversations (
 id uuid PRIMARY KEY,
 kind text NOT NULL CHECK(kind IN ('direct','group')),
 name text,
 direct_a uuid,
 direct_b uuid,
 next_ordinal bigint NOT NULL DEFAULT 1 CHECK(next_ordinal > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT issue25_direct_pair CHECK (
  (kind='direct' AND direct_a IS NOT NULL AND direct_b IS NOT NULL AND direct_a < direct_b AND name IS NULL)
  OR (kind='group' AND direct_a IS NULL AND direct_b IS NULL AND name IS NOT NULL AND octet_length(name) BETWEEN 1 AND 256)
 )
);
CREATE UNIQUE INDEX issue25_direct_unique ON issue25_conversations(direct_a,direct_b) WHERE kind='direct';
CREATE TABLE issue25_memberships (
 epoch uuid PRIMARY KEY,
 conversation_id uuid NOT NULL REFERENCES issue25_conversations(id),
 account_id uuid NOT NULL,
 role text NOT NULL CHECK(role IN ('owner','admin','member')),
 join_boundary bigint NOT NULL CHECK(join_boundary > 0),
 leave_boundary bigint,
 CHECK(leave_boundary IS NULL OR leave_boundary >= join_boundary)
);
CREATE UNIQUE INDEX issue25_active_member ON issue25_memberships(conversation_id,account_id) WHERE leave_boundary IS NULL;
CREATE INDEX issue25_membership_history ON issue25_memberships(conversation_id,account_id,join_boundary);
CREATE UNIQUE INDEX issue25_one_owner ON issue25_memberships(conversation_id) WHERE role='owner' AND leave_boundary IS NULL;
CREATE TABLE issue25_messages (
 id uuid PRIMARY KEY,
 conversation_id uuid NOT NULL REFERENCES issue25_conversations(id),
 ordinal bigint NOT NULL CHECK(ordinal > 0),
 sender_id uuid NOT NULL,
 idempotency_key uuid NOT NULL,
 body text NOT NULL DEFAULT '',
 document_id text,
 document_title text,
 canonical_payload bytea NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(conversation_id,ordinal),
 UNIQUE(conversation_id,sender_id,idempotency_key),
 CHECK(octet_length(body)<=16384),
 CHECK((document_id IS NULL AND document_title IS NULL)
 OR (document_id IS NOT NULL AND document_title IS NOT NULL AND length(btrim(document_id))>0 AND length(btrim(document_title))>0)),
 CHECK(length(btrim(body))>0 OR document_id IS NOT NULL)
);
CREATE INDEX issue25_message_history ON issue25_messages(conversation_id,ordinal);
CREATE TABLE issue25_user_state (
 conversation_id uuid NOT NULL REFERENCES issue25_conversations(id),
 account_id uuid NOT NULL,
 hidden boolean NOT NULL DEFAULT false,
 last_read_ordinal bigint NOT NULL DEFAULT 0,
 PRIMARY KEY(conversation_id,account_id)
);
CREATE TABLE issue25_publish_attempts (
 attempt_id uuid PRIMARY KEY,
 conversation_id uuid NOT NULL,
 viewer_id uuid NOT NULL,
 membership_epoch uuid NOT NULL REFERENCES issue25_memberships(epoch),
 message_id uuid NOT NULL REFERENCES issue25_messages(id),
 message_ordinal bigint NOT NULL,
 wake_token text NOT NULL,
 opaque_channel text NOT NULL,
 state text NOT NULL DEFAULT 'REGISTERED' CHECK(state IN ('REGISTERED','CLAIMED','SUCCESS','UNKNOWN','FAILED')),
 created_at timestamptz NOT NULL DEFAULT now(),
 claimed_at timestamptz,
 UNIQUE(conversation_id,viewer_id,membership_epoch,message_id),
 CHECK(octet_length(wake_token)>0 AND octet_length(opaque_channel)>0)
);
-- Immutable logical binding: only state and claimed_at can change.
-- +goose StatementBegin
CREATE FUNCTION issue25_attempt_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (OLD.attempt_id,OLD.conversation_id,OLD.viewer_id,OLD.membership_epoch,OLD.message_id,OLD.message_ordinal,OLD.wake_token,OLD.opaque_channel)
 IS DISTINCT FROM
 (NEW.attempt_id,NEW.conversation_id,NEW.viewer_id,NEW.membership_epoch,NEW.message_id,NEW.message_ordinal,NEW.wake_token,NEW.opaque_channel)
 THEN RAISE EXCEPTION 'IMMUTABLE_ATTEMPT' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER issue25_attempt_guard BEFORE UPDATE ON issue25_publish_attempts
FOR EACH ROW EXECUTE FUNCTION issue25_attempt_immutable();
-- +goose Down
DROP TABLE issue25_publish_attempts;
DROP FUNCTION issue25_attempt_immutable();
DROP TABLE issue25_user_state;
DROP TABLE issue25_messages;
DROP TABLE issue25_memberships;
DROP TABLE issue25_conversations;
