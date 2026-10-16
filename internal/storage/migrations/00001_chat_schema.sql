-- +goose Up
-- DP-M07 schema: assignments are explicit; opening a chat does not create a conversation.
-- UUIDs are opaque BFE Drive account identifiers; there are no cross-service FKs.
CREATE TABLE chat_assignments (
    owner_account_id uuid PRIMARY KEY,
    recipient_account_id uuid NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE chat_conversations (
    owner_account_id uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE chat_messages (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_account_id uuid NOT NULL REFERENCES chat_conversations(owner_account_id),
    sender_account_id uuid NOT NULL,
    recipient_account_id uuid NOT NULL,
    idempotency_key uuid NOT NULL,
    body text NOT NULL DEFAULT '',
    document_id text,
    document_title text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chat_message_nonempty CHECK (length(btrim(body)) > 0 OR document_id IS NOT NULL),
    CONSTRAINT chat_document_pair CHECK (
        (document_id IS NULL AND document_title IS NULL) OR
        (document_id IS NOT NULL AND document_title IS NOT NULL
         AND length(btrim(document_id)) > 0 AND length(btrim(document_title)) > 0)
    ),
    CONSTRAINT chat_message_idempotency UNIQUE (owner_account_id, sender_account_id, idempotency_key)
);

CREATE INDEX chat_messages_history_idx ON chat_messages (owner_account_id, id);

-- +goose Down
DROP TABLE chat_messages;
DROP TABLE chat_conversations;
DROP TABLE chat_assignments;
