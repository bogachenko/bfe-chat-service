-- +goose Up
-- FC-11 and FC-12 share one transaction-scoped lock per owner.
-- The lock is taken BEFORE reading an assignment or replaying a send.
-- ponytail: hash collisions serialize unrelated owners; upgrade to a dedicated
-- per-owner lock row only if lock contention becomes measurable.
-- +goose StatementBegin
CREATE FUNCTION chat_commit_assignment(p_owner uuid, p_recipient uuid)
RETURNS chat_assignments
LANGUAGE plpgsql
AS $$
DECLARE
    result chat_assignments%ROWTYPE;
BEGIN
    IF p_owner IS NULL OR p_recipient IS NULL THEN
        RAISE EXCEPTION 'ASSIGNMENT_REJECTED' USING ERRCODE = '22023';
    END IF;

    PERFORM pg_advisory_xact_lock(hashtextextended(p_owner::text, 0));

    INSERT INTO chat_assignments (owner_account_id, recipient_account_id)
    VALUES (p_owner, p_recipient)
    ON CONFLICT (owner_account_id)
    DO UPDATE SET recipient_account_id = EXCLUDED.recipient_account_id,
                  updated_at = now()
    RETURNING * INTO result;

    RETURN result;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION chat_commit_message(
    p_owner uuid,
    p_sender uuid,
    p_idempotency_key uuid,
    p_body text,
    p_document_id text,
    p_document_title text
)
RETURNS TABLE(message_id bigint, recipient_account_id uuid, replayed boolean)
LANGUAGE plpgsql
AS $$
DECLARE
    existing_id bigint;
    existing_recipient uuid;
    current_recipient uuid;
BEGIN
    IF p_owner IS NULL OR p_sender IS NULL OR p_idempotency_key IS NULL THEN
        RAISE EXCEPTION 'VALIDATION_ERROR' USING ERRCODE = '22023';
    END IF;

    -- SERIAL ORDER: both functions take the same lock for this owner.
    PERFORM pg_advisory_xact_lock(hashtextextended(p_owner::text, 0));

    -- Replay must precede assignment lookup: reassignment cannot rewrite
    -- the original recipient and loss of assignment must not break retries.
    SELECT m.id, m.recipient_account_id
      INTO existing_id, existing_recipient
      FROM chat_messages AS m
     WHERE m.owner_account_id = p_owner
       AND m.sender_account_id = p_sender
       AND m.idempotency_key = p_idempotency_key;

    IF FOUND THEN
        RETURN QUERY SELECT existing_id, existing_recipient, true;
        RETURN;
    END IF;

    IF (coalesce(length(btrim(p_body)), 0) = 0 AND p_document_id IS NULL)
       OR ((p_document_id IS NULL) <> (p_document_title IS NULL))
       OR (p_document_id IS NOT NULL AND
           (length(btrim(p_document_id)) = 0 OR length(btrim(p_document_title)) = 0)) THEN
        RAISE EXCEPTION 'VALIDATION_ERROR' USING ERRCODE = '22023';
    END IF;

    SELECT a.recipient_account_id INTO current_recipient
      FROM chat_assignments AS a WHERE a.owner_account_id = p_owner;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'NO_ASSIGNMENT' USING ERRCODE = 'P0001';
    END IF;

    -- Conversation is created only by the first successfully committed send.
    INSERT INTO chat_conversations (owner_account_id) VALUES (p_owner)
    ON CONFLICT (owner_account_id) DO NOTHING;

    INSERT INTO chat_messages (
        owner_account_id, sender_account_id, recipient_account_id,
        idempotency_key, body, document_id, document_title
    ) VALUES (
        p_owner, p_sender, current_recipient,
        p_idempotency_key, coalesce(p_body, ''), p_document_id, p_document_title
    ) RETURNING id INTO existing_id;

    RETURN QUERY SELECT existing_id, current_recipient, false;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION chat_commit_message(uuid, uuid, uuid, text, text, text);
DROP FUNCTION chat_commit_assignment(uuid, uuid);
