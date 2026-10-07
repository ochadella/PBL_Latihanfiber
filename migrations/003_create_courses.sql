CREATE TABLE IF NOT EXISTS courses (
    id          SERIAL PRIMARY KEY,
    kode_mk     VARCHAR(20)  NOT NULL UNIQUE,
    nama_mk     VARCHAR(150) NOT NULL,
    sks         INT          NOT NULL CHECK (sks > 0),
    semester    INT          NOT NULL CHECK (semester BETWEEN 1 AND 8),
    kuota       INT          NOT NULL CHECK (kuota >= 0),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
