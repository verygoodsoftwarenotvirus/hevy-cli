package archive

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oneExercise is a single exercise with one fully populated set and one entirely empty set, so
// every test workout exercises both the populated and the NULL column paths.
const oneExercise = `{
	"index": 0,
	"title": "Squat (Barbell)",
	"notes": "felt good",
	"exercise_template_id": "TPL1",
	"supersets_id": null,
	"sets": [
		{"index":0,"type":"normal","weight_kg":100.5,"reps":5,"rpe":8,"distance_meters":null,"duration_seconds":null,"custom_metric":null},
		{"index":1,"type":"warmup","weight_kg":null,"reps":null,"rpe":null,"distance_meters":null,"duration_seconds":null,"custom_metric":null}
	]
}`

// workoutJSON renders one workout as the API would, an hour long, with extra spliced in verbatim.
func workoutJSON(t *testing.T, id, start, extra string) string {
	t.Helper()

	st, err := time.Parse(time.RFC3339, start)
	require.NoError(t, err)

	return fmt.Sprintf(
		`{"id":%q,"title":"W-%s","description":"","start_time":%q,"end_time":%q,`+
			`"created_at":%q,"updated_at":%q,"routine_id":null,"is_private":false,"exercises":[%s]%s}`,
		id, id, start, st.Add(time.Hour).Format(time.RFC3339), start, start, oneExercise, extra,
	)
}

// newFakeAPI serves the given pages of raw workout JSON, newest-first, and counts page requests.
func newFakeAPI(t *testing.T, pages [][]string) (*hevy.Client, *atomic.Int64) {
	t.Helper()

	var requests atomic.Int64

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/workouts", func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)

		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page < 1 {
			page = 1
		}
		if page > len(pages) {
			http.Error(w, "no such page", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"page":%d,"page_count":%d,"workouts":[%s]}`,
			page, len(pages), strings.Join(pages[page-1], ","))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return hevy.NewClient("test-key", hevy.WithBaseURL(srv.URL)), &requests
}

// openArchive opens a written archive read-only for assertions.
func openArchive(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})

	return db
}

func queryInt(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()

	var n int
	require.NoError(t, db.QueryRowContext(t.Context(), query, args...).Scan(&n))

	return n
}

func TestRun_ArchivesOnlyTheRequestedYear(t *testing.T) {
	t.Parallel()

	pages := [][]string{
		{
			workoutJSON(t, "w-2026", "2026-01-05T10:00:00Z", ""), // newer than the year: skipped
			workoutJSON(t, "w-a", "2025-06-02T10:00:00Z", ""),
		},
		{
			workoutJSON(t, "w-b", "2025-06-01T10:00:00Z", ""),
			workoutJSON(t, "w-c", "2025-01-02T10:00:00Z", ""),
		},
		{
			workoutJSON(t, "w-2024", "2024-12-31T10:00:00Z", ""), // older: stops the scan
			workoutJSON(t, "w-old", "2024-11-30T10:00:00Z", ""),
		},
		{
			workoutJSON(t, "w-ancient", "2023-01-01T10:00:00Z", ""),
		},
	}

	client, requests := newFakeAPI(t, pages)
	out := filepath.Join(t.TempDir(), "2025.db")

	result, err := Run(t.Context(), client, Options{
		Location: time.UTC,
		Out:      out,
		Year:     2025,
	}, nil)
	require.NoError(t, err)

	assert.Equal(t, 3, result.Workouts)
	assert.Equal(t, 3, result.Exercises)
	assert.Equal(t, 6, result.Sets)
	assert.Equal(t, "2025-01-02", result.First.Format(time.DateOnly))
	assert.Equal(t, "2025-06-02", result.Last.Format(time.DateOnly))
	assert.Equal(t, 7, result.Fetched)
	assert.False(t, result.OutOfOrder, "this fixture is genuinely newest-first")

	// Every page must be read: the API does not guarantee start-time order, so an early stop would
	// silently truncate the archive.
	assert.Equal(t, int64(4), requests.Load(), "all pages should have been fetched")

	db := openArchive(t, out)
	assert.Equal(t, 3, queryInt(t, db, "SELECT COUNT(*) FROM workouts"))
	assert.Equal(t, 6, queryInt(t, db, "SELECT COUNT(*) FROM workout_sets"))
	assert.Equal(t, 0, queryInt(t, db, "SELECT COUNT(*) FROM workouts WHERE id IN ('w-2026','w-2024','w-old')"))
	assert.Equal(t, 3600, queryInt(t, db, "SELECT duration_seconds FROM workouts WHERE id = 'w-a'"))
	assert.Equal(t, 2025, queryInt(t, db, "SELECT CAST(value AS INTEGER) FROM archive_meta WHERE key = 'year'"))
}

func TestRun_PreservesNullsAndRawJSON(t *testing.T) {
	t.Parallel()

	pages := [][]string{{
		workoutJSON(t, "w-a", "2025-06-02T10:00:00Z", `,"a_field_we_do_not_model":"keep me"`),
		workoutJSON(t, "w-old", "2024-01-01T10:00:00Z", ""),
	}}

	client, _ := newFakeAPI(t, pages)
	out := filepath.Join(t.TempDir(), "2025.db")

	_, err := Run(t.Context(), client, Options{Location: time.UTC, Out: out, Year: 2025}, nil)
	require.NoError(t, err)

	db := openArchive(t, out)

	// An unrecorded weight must stay NULL, not collapse to 0, or averages would silently skew.
	assert.Equal(t, 1, queryInt(t, db,
		"SELECT COUNT(*) FROM workout_sets WHERE set_index = 1 AND weight_kg IS NULL AND reps IS NULL AND rpe IS NULL"))
	assert.Equal(t, 1, queryInt(t, db,
		"SELECT COUNT(*) FROM workout_sets WHERE set_index = 0 AND weight_kg = 100.5 AND reps = 5 AND rpe = 8"))
	assert.Equal(t, 1, queryInt(t, db, "SELECT COUNT(*) FROM workouts WHERE routine_id IS NULL"))

	var raw string
	require.NoError(t, db.QueryRowContext(t.Context(),
		"SELECT raw_json FROM workouts WHERE id = 'w-a'").Scan(&raw))
	assert.Contains(t, raw, "a_field_we_do_not_model", "raw JSON should survive fields the struct drops")

	// The convenience view should join cleanly and compute volume.
	assert.Equal(t, 1, queryInt(t, db,
		"SELECT COUNT(*) FROM v_sets WHERE exercise_title = 'Squat (Barbell)' AND volume_kg IS NOT NULL"))
}

// TestRun_CapturesWorkoutsAfterAnOlderOne reproduces the real API's behavior: the list endpoint is
// not ordered by start_time, so in-year workouts appear after out-of-year ones. A scan that stopped
// at the first older workout would archive only the first few and silently drop the rest.
func TestRun_CapturesWorkoutsAfterAnOlderOne(t *testing.T) {
	t.Parallel()

	pages := [][]string{
		{
			workoutJSON(t, "w-dec", "2024-12-31T10:00:00Z", ""),
			workoutJSON(t, "w-nov", "2024-11-20T10:00:00Z", ""),
		},
		{
			// An edited or imported workout drops an old session in mid-list. The early-stop scan
			// gave up here, which is why a full year came back as a scattered handful.
			workoutJSON(t, "w-ancient", "2019-05-05T10:00:00Z", ""),
			workoutJSON(t, "w-feb", "2024-02-10T10:00:00Z", ""),
		},
		{
			workoutJSON(t, "w-jan-a", "2024-01-30T10:00:00Z", ""),
			workoutJSON(t, "w-jan-b", "2024-01-08T10:00:00Z", ""),
		},
	}

	client, requests := newFakeAPI(t, pages)
	out := filepath.Join(t.TempDir(), "2024.db")

	result, err := Run(t.Context(), client, Options{Location: time.UTC, Out: out, Year: 2024}, nil)
	require.NoError(t, err)

	assert.Equal(t, 5, result.Workouts, "the January workouts after the 2019 one must be archived")
	assert.Equal(t, int64(3), requests.Load(), "all pages must be scanned")
	assert.True(t, result.OutOfOrder, "the fixture is deliberately out of start-time order")
	assert.Equal(t, "2024-01-08", result.First.Format(time.DateOnly))

	db := openArchive(t, out)
	assert.Equal(t, 0, queryInt(t, db, "SELECT COUNT(*) FROM workouts WHERE id = 'w-ancient'"))
	assert.Equal(t, 2, queryInt(t, db, "SELECT COUNT(*) FROM workouts WHERE local_date LIKE '2024-01-%'"))
}

func TestRun_EnforcesForeignKeys(t *testing.T) {
	t.Parallel()

	pages := [][]string{{
		workoutJSON(t, "w-a", "2025-06-02T10:00:00Z", ""),
		workoutJSON(t, "w-old", "2024-01-01T10:00:00Z", ""),
	}}

	client, _ := newFakeAPI(t, pages)
	out := filepath.Join(t.TempDir(), "2025.db")

	_, err := Run(t.Context(), client, Options{Location: time.UTC, Out: out, Year: 2025}, nil)
	require.NoError(t, err)

	// The archive declares real foreign keys rather than bare columns, so a reader that opens the
	// file with foreign_keys on cannot introduce orphans. The import uses this same DSN.
	db, err := sql.Open("sqlite", "file:"+out+"?_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})

	_, err = db.ExecContext(t.Context(),
		`INSERT INTO workout_sets (workout_id, exercise_index, set_index, type)
		 VALUES ('does-not-exist', 0, 0, 'normal')`)
	require.Error(t, err, "an orphaned set must be rejected")
	assert.Contains(t, strings.ToLower(err.Error()), "foreign key")
}

func TestRun_BucketsByLocalTimezone(t *testing.T) {
	t.Parallel()

	// 02:00 UTC on Jan 1 is still 2025-12-31 in New York, so this belongs to the 2025 archive.
	pages := [][]string{{
		workoutJSON(t, "w-newyear", "2026-01-01T02:00:00Z", ""),
		workoutJSON(t, "w-old", "2024-01-01T10:00:00Z", ""),
	}}

	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	client, _ := newFakeAPI(t, pages)
	out := filepath.Join(t.TempDir(), "2025.db")

	result, err := Run(t.Context(), client, Options{Location: loc, Out: out, Year: 2025}, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Workouts)

	db := openArchive(t, out)
	var localDate string
	require.NoError(t, db.QueryRowContext(t.Context(),
		"SELECT local_date FROM workouts WHERE id = 'w-newyear'").Scan(&localDate))
	assert.Equal(t, "2025-12-31", localDate)

	var tz string
	require.NoError(t, db.QueryRowContext(t.Context(),
		"SELECT value FROM archive_meta WHERE key = 'timezone'").Scan(&tz))
	assert.Equal(t, "America/New_York", tz)
}

func TestRun_RefusesExistingFileWithoutForce(t *testing.T) {
	t.Parallel()

	pages := [][]string{{
		workoutJSON(t, "w-a", "2025-06-02T10:00:00Z", ""),
		workoutJSON(t, "w-old", "2024-01-01T10:00:00Z", ""),
	}}

	client, _ := newFakeAPI(t, pages)
	out := filepath.Join(t.TempDir(), "2025.db")
	require.NoError(t, os.WriteFile(out, []byte("precious"), 0o600))

	_, err := Run(t.Context(), client, Options{Location: time.UTC, Out: out, Year: 2025}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")

	// The existing file must be untouched.
	contents, readErr := os.ReadFile(out)
	require.NoError(t, readErr)
	assert.Equal(t, "precious", string(contents))

	result, err := Run(t.Context(), client, Options{Location: time.UTC, Out: out, Year: 2025, Force: true}, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Workouts)

	db := openArchive(t, out)
	assert.Equal(t, 1, queryInt(t, db, "SELECT COUNT(*) FROM workouts"))
}

func TestRun_LeavesNoFileWhenTheAPIFails(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := hevy.NewClient("test-key", hevy.WithBaseURL(srv.URL))
	out := filepath.Join(t.TempDir(), "2025.db")

	_, err := Run(t.Context(), client, Options{Location: time.UTC, Out: out, Year: 2025}, nil)
	require.Error(t, err)

	assert.NoFileExists(t, out, "a failed run must not leave a partial archive")
	assert.NoFileExists(t, out+".tmp", "a failed run must clean up its temp file")
}

func TestRun_ReportsProgressAndHandlesEmptyYears(t *testing.T) {
	t.Parallel()

	pages := [][]string{{
		workoutJSON(t, "w-old", "2024-01-01T10:00:00Z", ""),
	}}

	client, _ := newFakeAPI(t, pages)
	out := filepath.Join(t.TempDir(), "2025.db")

	var calls int
	result, err := Run(t.Context(), client, Options{Location: time.UTC, Out: out, Year: 2025},
		func(fetched, kept int) {
			calls++
			assert.Positive(t, fetched)
			assert.Zero(t, kept)
		})
	require.NoError(t, err)

	assert.Equal(t, 1, calls)
	assert.Zero(t, result.Workouts)
	assert.Equal(t, 1, result.Fetched)

	// An empty year still produces a valid, queryable archive.
	db := openArchive(t, out)
	assert.Equal(t, 0, queryInt(t, db, "SELECT COUNT(*) FROM workouts"))
	assert.Equal(t, 0, queryInt(t, db, "SELECT CAST(value AS INTEGER) FROM archive_meta WHERE key = 'workout_count'"))
}

func TestRun_RejectsBadOptions(t *testing.T) {
	t.Parallel()

	client, _ := newFakeAPI(t, [][]string{{}})

	_, err := Run(t.Context(), client, Options{Out: filepath.Join(t.TempDir(), "x.db")}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "year")

	_, err = Run(t.Context(), client, Options{Year: 2025}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output path")

	_, err = Run(t.Context(), client, Options{
		Year: 2025,
		Out:  filepath.Join(t.TempDir(), "no-such-dir", "x.db"),
	}, nil)
	require.Error(t, err)
}

func TestBackoff(t *testing.T) {
	t.Parallel()

	rt := &retryTransport{baseBackoff: time.Second, attempts: defaultAttempts}

	assert.Equal(t, time.Second, rt.backoff(1, 0))
	assert.Equal(t, 2*time.Second, rt.backoff(2, 0))
	assert.Equal(t, 4*time.Second, rt.backoff(3, 0))
	assert.Equal(t, maxBackoff, rt.backoff(20, 0), "growth must be capped")
	assert.Equal(t, 90*time.Second, rt.backoff(1, 90*time.Second), "a longer server hint wins")
	assert.Equal(t, time.Second, rt.backoff(1, 100*time.Millisecond), "a shorter server hint is ignored")
}

func TestRetryTransport_RetriesRateLimits(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"page":1,"page_count":1,"workouts":[]}`)
	}))
	t.Cleanup(srv.Close)

	// A tiny backoff keeps the test fast while still exercising the sleep path.
	client := hevy.NewClient("test-key", hevy.WithBaseURL(srv.URL), hevy.WithHTTPClient(&http.Client{
		Transport: &retryTransport{base: http.DefaultTransport, attempts: 5, baseBackoff: time.Millisecond},
	}))

	count, err := client.GetWorkoutCount(t.Context())
	require.NoError(t, err)
	assert.Zero(t, count)
	assert.Equal(t, int64(3), requests.Load(), "should have retried twice before succeeding")
}

func TestRetryTransport_GivesUpAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := hevy.NewClient("test-key", hevy.WithBaseURL(srv.URL), hevy.WithHTTPClient(&http.Client{
		Transport: &retryTransport{base: http.DefaultTransport, attempts: 3, baseBackoff: time.Millisecond},
	}))

	_, err := client.GetWorkoutCount(t.Context())
	require.Error(t, err)
	assert.Equal(t, int64(3), requests.Load())
}
