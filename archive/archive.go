// Package archive exports a year of Hevy workout history into a self-contained SQLite file.
package archive

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite".
)

// Options configures a single archive run.
type Options struct {
	// Location determines which calendar year a workout belongs to. Defaults to time.Local.
	Location *time.Location
	// Out is the path of the SQLite file to create.
	Out string
	// Year selects the workouts to archive, by local start time.
	Year int
	// Force permits overwriting an existing Out.
	Force bool
}

// Result summarizes what an archive run wrote.
type Result struct {
	// First and Last are the local start times of the earliest and latest archived workouts.
	First time.Time
	Last  time.Time
	// Fetched counts workouts read from the API, including those outside the year.
	Fetched   int
	Workouts  int
	Exercises int
	Sets      int
	// OutOfOrder reports that the API returned workouts whose start times were not descending,
	// meaning a scan that stopped at the first out-of-window workout would have missed data.
	OutOfOrder bool
}

// ProgressFunc reports how many workouts have been fetched from the API and how many of those fell
// inside the requested year. It is called periodically during a run and may be nil.
type ProgressFunc func(fetched, kept int)

// Run archives every workout that started in opts.Year into a fresh SQLite file at opts.Out.
//
// The database is built at a temporary path and renamed into place only after a clean commit, so an
// interrupted or failed run never leaves a partial archive behind.
func Run(ctx context.Context, client *hevy.Client, opts Options, progress ProgressFunc) (result *Result, err error) {
	loc := opts.Location
	if loc == nil {
		// Deliberate: a workout belongs to the year it was logged in where the lifter was.
		loc = time.Local //nolint:gosmopolitan // local time is the intended default for year bucketing.
	}

	if opts.Year <= 0 {
		return nil, fmt.Errorf("year must be positive, got %d", opts.Year)
	}
	if opts.Out == "" {
		return nil, errors.New("output path is required")
	}

	if !opts.Force {
		switch _, statErr := os.Stat(opts.Out); {
		case statErr == nil:
			return nil, fmt.Errorf("%s already exists; pass --force to overwrite it", opts.Out)
		case !errors.Is(statErr, fs.ErrNotExist):
			return nil, fmt.Errorf("checking output path %s: %w", opts.Out, statErr)
		}
	}

	tmpPath := opts.Out + ".tmp"
	if rmErr := os.Remove(tmpPath); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("clearing stale temp file %s: %w", tmpPath, rmErr)
	}

	// Leave nothing behind if anything below fails.
	defer func() {
		if err == nil {
			return
		}
		if rmErr := os.Remove(tmpPath); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("removing temp file %s: %w", tmpPath, rmErr))
		}
	}()

	result, err = build(ctx, client, opts, loc, tmpPath, progress)
	if err != nil {
		return nil, err
	}

	if err = os.Rename(tmpPath, opts.Out); err != nil {
		return nil, fmt.Errorf("moving archive into place at %s: %w", opts.Out, err)
	}

	return result, nil
}

// build creates the database at path and fills it, closing it before returning.
func build(
	ctx context.Context,
	client *hevy.Client,
	opts Options,
	loc *time.Location,
	path string,
	progress ProgressFunc,
) (result *Result, err error) {
	// foreign_keys is a per-connection pragma, so it goes in the DSN rather than a stray Exec that
	// would only configure whichever pooled connection happened to serve it. Capping the pool at one
	// connection keeps this a single writer, which is all an import needs.
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing %s: %w", path, closeErr))
			result = nil
		}
	}()

	if _, err = db.ExecContext(ctx, schema); err != nil {
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	result, err = ingest(ctx, db, client, opts, loc, progress)
	if err != nil {
		return nil, err
	}

	// VACUUM must run outside the import transaction.
	if _, err = db.ExecContext(ctx, "VACUUM"); err != nil {
		return nil, fmt.Errorf("compacting archive: %w", err)
	}

	return result, nil
}

// ingest fetches the year's workouts and writes them in a single transaction.
func ingest(
	ctx context.Context,
	db *sql.DB,
	client *hevy.Client,
	opts Options,
	loc *time.Location,
	progress ProgressFunc,
) (result *Result, err error) {
	start := time.Date(opts.Year, time.January, 1, 0, 0, 0, 0, loc)
	end := start.AddDate(1, 0, 0)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}

	committed := false

	defer func() {
		if committed {
			return
		}
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rolling back: %w", rbErr))
			result = nil
		}
	}()

	stmts, err := prepareStatements(ctx, tx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if closeErr := stmts.close(); closeErr != nil {
			err = errors.Join(err, closeErr)
			result = nil
		}
	}()

	res := &Result{}

	// The list endpoint is not ordered by start_time: editing or importing a workout moves it, so a
	// 2024 session can appear after a 2019 one. Stopping at the first out-of-window workout would
	// silently truncate the archive, so every page is read. OutOfOrder records whether that actually
	// happened, which is what justifies the full scan.
	var prevStart time.Time

	for raw, listErr := range client.ListWorkoutsRaw(ctx) {
		if listErr != nil {
			return nil, fmt.Errorf("listing workouts: %w", listErr)
		}

		var w hevy.Workout
		if unmarshalErr := json.Unmarshal(raw, &w); unmarshalErr != nil {
			return nil, fmt.Errorf("decoding workout: %w", unmarshalErr)
		}

		res.Fetched++
		if !prevStart.IsZero() && w.StartTime.After(prevStart) {
			res.OutOfOrder = true
		}
		prevStart = w.StartTime

		if progress != nil {
			progress(res.Fetched, res.Workouts)
		}

		if w.StartTime.Before(start) || !w.StartTime.Before(end) {
			continue
		}

		if err = insertWorkoutRow(ctx, stmts, &w, raw, loc, res); err != nil {
			return nil, err
		}
	}

	if err = writeMeta(ctx, stmts.meta, opts, loc, res); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing: %w", err)
	}
	committed = true

	return res, nil
}

// statements holds the prepared inserts used for every row of an import.
type statements struct {
	workout  *sql.Stmt
	exercise *sql.Stmt
	set      *sql.Stmt
	meta     *sql.Stmt
}

func prepareStatements(ctx context.Context, tx *sql.Tx) (*statements, error) {
	var (
		s   statements
		err error
	)

	prepares := []struct {
		dest  **sql.Stmt
		name  string
		query string
	}{
		{&s.workout, "workout", insertWorkout},
		{&s.exercise, "exercise", insertExercise},
		{&s.set, "set", insertSet},
		{&s.meta, "meta", insertMeta},
	}

	for i := range prepares {
		p := &prepares[i]

		//nolint:sqlclosecheck // every statement is closed by statements.close, deferred in ingest.
		*p.dest, err = tx.PrepareContext(ctx, p.query)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("preparing %s insert: %w", p.name, err), s.close())
		}
	}

	return &s, nil
}

func (s *statements) close() error {
	var errs []error
	for _, stmt := range []*sql.Stmt{s.workout, s.exercise, s.set, s.meta} {
		if stmt == nil {
			continue
		}
		if err := stmt.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing statement: %w", err))
		}
	}
	return errors.Join(errs...)
}

// insertWorkoutRow writes a workout and its exercises and sets, updating res as it goes.
func insertWorkoutRow(
	ctx context.Context,
	stmts *statements,
	w *hevy.Workout,
	raw json.RawMessage,
	loc *time.Location,
	res *Result,
) error {
	localStart := w.StartTime.In(loc)

	if _, err := stmts.workout.ExecContext(ctx,
		w.ID,
		w.Title,
		w.Description,
		w.StartTime.UTC().Format(time.RFC3339),
		w.EndTime.UTC().Format(time.RFC3339),
		w.CreatedAt.UTC().Format(time.RFC3339),
		w.UpdatedAt.UTC().Format(time.RFC3339),
		localStart.Format(time.DateOnly),
		int64(w.EndTime.Sub(w.StartTime).Seconds()),
		w.RoutineID,
		w.IsPrivate,
		string(raw),
	); err != nil {
		return fmt.Errorf("inserting workout %s: %w", w.ID, err)
	}

	res.Workouts++
	if res.First.IsZero() || localStart.Before(res.First) {
		res.First = localStart
	}
	if localStart.After(res.Last) {
		res.Last = localStart
	}

	for i := range w.Exercises {
		e := &w.Exercises[i]

		if _, err := stmts.exercise.ExecContext(ctx,
			w.ID, e.Index, e.Title, e.Notes, e.ExerciseTemplateID, e.SupersetID,
		); err != nil {
			return fmt.Errorf("inserting exercise %d of workout %s: %w", e.Index, w.ID, err)
		}
		res.Exercises++

		for j := range e.Sets {
			st := &e.Sets[j]

			if _, err := stmts.set.ExecContext(ctx,
				w.ID, e.Index, st.Index, string(st.Type),
				st.WeightKg, st.Reps, st.DistanceMeters, st.DurationSeconds, st.RPE, st.CustomMetric,
			); err != nil {
				return fmt.Errorf("inserting set %d of exercise %d of workout %s: %w", st.Index, e.Index, w.ID, err)
			}
			res.Sets++
		}
	}

	return nil
}

// writeMeta records how and when the archive was produced.
func writeMeta(ctx context.Context, stmt *sql.Stmt, opts Options, loc *time.Location, res *Result) error {
	rows := [][2]string{
		{"schema_version", schemaVersion},
		{"year", strconv.Itoa(opts.Year)},
		{"timezone", loc.String()},
		{"archived_at", time.Now().UTC().Format(time.RFC3339)},
		{"workout_count", strconv.Itoa(res.Workouts)},
		{"exercise_count", strconv.Itoa(res.Exercises)},
		{"set_count", strconv.Itoa(res.Sets)},
	}

	for i := range rows {
		row := &rows[i]
		if _, err := stmt.ExecContext(ctx, row[0], row[1]); err != nil {
			return fmt.Errorf("writing archive metadata %q: %w", row[0], err)
		}
	}

	return nil
}
