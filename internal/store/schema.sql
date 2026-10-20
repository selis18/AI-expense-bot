CREATE TABLE IF NOT EXISTS schema_migrations (
 version INTEGER PRIMARY KEY,
 applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE users (
 id BIGINT PRIMARY KEY CHECK (id > 0),
 display_name TEXT NOT NULL
);
CREATE TABLE expenses (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id),
 amount BIGINT NOT NULL CHECK (amount > 0),
 category TEXT NOT NULL CHECK (category IN ('еда','транспорт','жильё','развлечения','здоровье','покупки','другое')),
 spent_at DATE NOT NULL,
 comment TEXT NOT NULL DEFAULT '' CHECK (char_length(comment) <= 1000),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX expenses_owner_date ON expenses (user_id, spent_at DESC, id DESC);
CREATE TABLE dialog_sessions (
 user_id BIGINT PRIMARY KEY REFERENCES users(id),
 context JSONB NOT NULL DEFAULT '{}'::jsonb,
 expires_at TIMESTAMPTZ,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE processed_updates (
 update_id BIGINT PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id),
 result TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO schema_migrations(version) VALUES (1);
