-- REVIEW ONLY. Database: bfe_chat. Not executed.
-- Execute only after separate access approval, using a transaction opened READ ONLY.
BEGIN TRANSACTION READ ONLY;

-- 1. Exact object counts.
SELECT 'assignments' AS entity, COUNT(*) AS total FROM chat_assignments
UNION ALL SELECT 'conversations', COUNT(*) FROM chat_conversations
UNION ALL SELECT 'messages', COUNT(*) FROM chat_messages;

-- 2. Owners with messages addressed to multiple historical recipients.
SELECT owner_account_id, COUNT(*) AS messages,
       COUNT(DISTINCT recipient_account_id) AS historical_recipients,
       MIN(created_at) AS earliest_message, MAX(created_at) AS latest_message
FROM chat_messages
GROUP BY owner_account_id
HAVING COUNT(DISTINCT recipient_account_id) > 1
ORDER BY historical_recipients DESC, messages DESC, owner_account_id;

-- 3. Distinct recipient transitions in message ID order (NOT proof of assignment history).
WITH ordered AS (
  SELECT owner_account_id, id, recipient_account_id, created_at,
         LAG(recipient_account_id) OVER (PARTITION BY owner_account_id ORDER BY id) AS previous_recipient
  FROM chat_messages
)
SELECT owner_account_id, id, previous_recipient, recipient_account_id, created_at
FROM ordered
WHERE previous_recipient IS NOT NULL AND previous_recipient IS DISTINCT FROM recipient_account_id
ORDER BY owner_account_id, id;

-- 4. Symmetric message streams where each owner has the other account as recipient.
-- Counts possible ambiguity, not necessarily one logical direct chat.
WITH pairs AS (
  SELECT DISTINCT owner_account_id AS owner_id, recipient_account_id AS partner_id
  FROM chat_messages
  WHERE owner_account_id <> recipient_account_id
)
SELECT a.owner_id AS account_a, a.partner_id AS account_b
FROM pairs a
JOIN pairs b ON b.owner_id = a.partner_id AND b.partner_id = a.owner_id
WHERE a.owner_id < a.partner_id
ORDER BY account_a, account_b;

-- 5. Idempotency grouping (schema prevents exact duplicate tuples, not conflicting payload replay).
SELECT owner_account_id, sender_account_id, idempotency_key, COUNT(*) AS occurrences
FROM chat_messages
GROUP BY owner_account_id, sender_account_id, idempotency_key
HAVING COUNT(*) > 1
ORDER BY occurrences DESC;
-- A reused idempotency key with different attempted payload cannot be reconstructed
-- if the rejected/replayed request was never persisted.

-- 6. Document context completeness; existing CHECK constraint should prevent half-pairs.
SELECT
  COUNT(*) FILTER (WHERE document_id IS NOT NULL) AS messages_with_document,
  COUNT(*) FILTER (WHERE (document_id IS NULL) <> (document_title IS NULL)) AS broken_document_pairs,
  COUNT(*) FILTER (WHERE document_id IS NOT NULL AND
    (BTRIM(document_id) = '' OR BTRIM(document_title) = '')) AS blank_document_fields
FROM chat_messages;

-- 7. Current assignment disagrees with at least one saved recipient.
SELECT a.owner_account_id, a.recipient_account_id AS current_recipient,
       COUNT(*) AS messages_to_different_recipient
FROM chat_assignments a
JOIN chat_messages m ON m.owner_account_id = a.owner_account_id
WHERE m.recipient_account_id IS DISTINCT FROM a.recipient_account_id
GROUP BY a.owner_account_id, a.recipient_account_id
ORDER BY messages_to_different_recipient DESC;

-- 8. Messages/conversations without current assignment (can occur after lifecycle changes).
SELECT COUNT(*) AS conversations_without_assignment
FROM chat_conversations c
LEFT JOIN chat_assignments a USING (owner_account_id)
WHERE a.owner_account_id IS NULL;

-- 9. History integrity and owner of the message (no joins into BFE Drive).
SELECT COUNT(*) FILTER (WHERE sender_account_id = recipient_account_id) AS self_addressed,
       COUNT(*) FILTER (WHERE sender_account_id NOT IN (owner_account_id, recipient_account_id)) AS unexpected_sender
FROM chat_messages;

ROLLBACK;
