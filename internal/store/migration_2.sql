CREATE TABLE conversation_turns (
 update_id BIGINT PRIMARY KEY REFERENCES processed_updates(update_id),
 user_id BIGINT NOT NULL REFERENCES users(id),
 prompt TEXT NOT NULL,
 response TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX conversation_turns_user ON conversation_turns(user_id,update_id DESC);
INSERT INTO schema_migrations(version) VALUES (2);
