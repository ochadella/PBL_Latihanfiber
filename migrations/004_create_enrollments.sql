CREATE TABLE IF NOT EXISTS enrollments (
    id              SERIAL PRIMARY KEY,
    student_id      INT         NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    course_id       INT         NOT NULL REFERENCES courses(id)  ON DELETE CASCADE,
    tahun_akademik  VARCHAR(20) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_enrollment UNIQUE (student_id, course_id, tahun_akademik)
);

CREATE INDEX IF NOT EXISTS idx_enrollments_course ON enrollments(course_id);
