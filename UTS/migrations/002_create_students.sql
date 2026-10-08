CREATE TABLE IF NOT EXISTS students (
    id            SERIAL PRIMARY KEY,
    user_id       INT          NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    nim           CHAR(12)     NOT NULL UNIQUE,
    nama          VARCHAR(100) NOT NULL,
    prodi         VARCHAR(100) NOT NULL,
    angkatan      INT          NOT NULL CHECK (angkatan BETWEEN 1000 AND 9999),
    ipk_terakhir  NUMERIC(3,2) NOT NULL DEFAULT 0 CHECK (ipk_terakhir BETWEEN 0 AND 4),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ  NULL
);

CREATE INDEX IF NOT EXISTS idx_students_deleted_at ON students(deleted_at);
