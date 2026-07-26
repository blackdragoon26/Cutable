ALTER TABLE users
    ADD COLUMN IF NOT EXISTS demo_runs_used INTEGER NOT NULL DEFAULT 0;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_demo_runs_used_nonnegative;

ALTER TABLE users
    ADD CONSTRAINT users_demo_runs_used_nonnegative
    CHECK (demo_runs_used >= 0);
