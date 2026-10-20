-- +goose Up
-- Defensive DB boundary: application callers cannot bind notification attempts
-- to another message, conversation, viewer interval or retired epoch.
-- +goose StatementBegin
CREATE FUNCTION issue25_validate_attempt() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  v_msg record;
  v_member record;
  v_next bigint;
BEGIN
  -- Same transaction gate key and lock order as issue25.Store.tx.
  PERFORM pg_advisory_xact_lock(hashtextextended(NEW.conversation_id::text,25));
  SELECT next_ordinal INTO v_next FROM issue25_conversations
   WHERE id=NEW.conversation_id FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'CONVERSATION_NOT_FOUND' USING ERRCODE='23503'; END IF;

  SELECT conversation_id,ordinal INTO v_msg FROM issue25_messages WHERE id=NEW.message_id;
  IF NOT FOUND OR v_msg.conversation_id IS DISTINCT FROM NEW.conversation_id
     OR v_msg.ordinal IS DISTINCT FROM NEW.message_ordinal THEN
    RAISE EXCEPTION 'ATTEMPT_MESSAGE_MISMATCH' USING ERRCODE='23514';
  END IF;
  SELECT conversation_id,account_id,join_boundary,leave_boundary
    INTO v_member FROM issue25_memberships WHERE epoch=NEW.membership_epoch;
  IF NOT FOUND OR v_member.conversation_id IS DISTINCT FROM NEW.conversation_id
     OR v_member.account_id IS DISTINCT FROM NEW.viewer_id
     OR v_member.leave_boundary IS NOT NULL
     OR NEW.message_ordinal < v_member.join_boundary
     OR NEW.message_ordinal >= v_next THEN
    RAISE EXCEPTION 'ATTEMPT_EPOCH_MISMATCH' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER issue25_attempt_insert_guard BEFORE INSERT ON issue25_publish_attempts
FOR EACH ROW EXECUTE FUNCTION issue25_validate_attempt();

-- +goose Down
DROP TRIGGER issue25_attempt_insert_guard ON issue25_publish_attempts;
DROP FUNCTION issue25_validate_attempt();
