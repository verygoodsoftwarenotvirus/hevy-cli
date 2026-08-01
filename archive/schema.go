package archive

// schemaVersion is written to archive_meta so future readers can detect layout changes.
const schemaVersion = "1"

// schema is the complete DDL for an archive file, applied in a single Exec.
//
// Nullable columns mirror the API's pointer fields exactly: a set that never recorded a weight
// stores NULL rather than 0, so aggregates cannot silently count absent data as zero.
const schema = `
CREATE TABLE archive_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

CREATE TABLE workouts (
    id               TEXT PRIMARY KEY,
    title            TEXT NOT NULL,
    description      TEXT NOT NULL,
    start_time       TEXT NOT NULL,
    end_time         TEXT NOT NULL,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    local_date       TEXT NOT NULL,
    duration_seconds INTEGER NOT NULL,
    routine_id       TEXT,
    is_private       INTEGER NOT NULL,
    raw_json         TEXT NOT NULL
) STRICT;

CREATE TABLE workout_exercises (
    workout_id           TEXT NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    exercise_index       INTEGER NOT NULL,
    title                TEXT NOT NULL,
    notes                TEXT NOT NULL,
    exercise_template_id TEXT NOT NULL,
    superset_id          INTEGER,
    PRIMARY KEY (workout_id, exercise_index)
) STRICT;

CREATE TABLE workout_sets (
    workout_id       TEXT NOT NULL,
    exercise_index   INTEGER NOT NULL,
    set_index        INTEGER NOT NULL,
    type             TEXT NOT NULL,
    weight_kg        REAL,
    reps             INTEGER,
    distance_meters  REAL,
    duration_seconds INTEGER,
    rpe              REAL,
    custom_metric    REAL,
    PRIMARY KEY (workout_id, exercise_index, set_index),
    FOREIGN KEY (workout_id, exercise_index)
        REFERENCES workout_exercises(workout_id, exercise_index) ON DELETE CASCADE
) STRICT;

CREATE INDEX idx_workouts_local_date ON workouts(local_date);
CREATE INDEX idx_exercises_template  ON workout_exercises(exercise_template_id);
CREATE INDEX idx_exercises_title     ON workout_exercises(title);

CREATE VIEW v_sets AS
SELECT w.id            AS workout_id,
       w.local_date    AS local_date,
       w.title         AS workout_title,
       e.exercise_index,
       e.title         AS exercise_title,
       e.exercise_template_id,
       s.set_index,
       s.type,
       s.weight_kg,
       s.reps,
       s.rpe,
       s.distance_meters,
       s.duration_seconds,
       (s.weight_kg * s.reps) AS volume_kg
FROM workouts w
JOIN workout_exercises e ON e.workout_id = w.id
JOIN workout_sets s      ON s.workout_id = e.workout_id AND s.exercise_index = e.exercise_index;
`

const (
	insertWorkout = `INSERT INTO workouts
        (id, title, description, start_time, end_time, created_at, updated_at,
         local_date, duration_seconds, routine_id, is_private, raw_json)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	insertExercise = `INSERT INTO workout_exercises
        (workout_id, exercise_index, title, notes, exercise_template_id, superset_id)
        VALUES (?, ?, ?, ?, ?, ?)`

	insertSet = `INSERT INTO workout_sets
        (workout_id, exercise_index, set_index, type, weight_kg, reps,
         distance_meters, duration_seconds, rpe, custom_metric)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	insertMeta = `INSERT INTO archive_meta (key, value) VALUES (?, ?)`
)
