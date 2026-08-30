package strongimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
)

// strongAliases maps the names Strong used for timed exercises onto the names Hevy settled on.
//
// Only exercises Strong actually times need an entry: the restore reads the Seconds column, and
// every other exercise leaves it zero. The list is deliberately explicit rather than fuzzy-matched,
// because a wrong guess here writes a real number onto the wrong exercise, which is worse than
// leaving a blank the operator can see.
var strongAliases = map[string]string{
	"Plank":              "Plank",
	"Side Plank":         "Side Plank",
	"Reverse Plank":      "Reverse Plank",
	"Stretching":         "Stretching",
	"Dead Hang":          "Dead Hang",
	"Elliptical Machine": "Elliptical Trainer",
	"Cycling (Indoor)":   "Cycling",
	"Treadmill":          "Treadmill",
	"Spinning":           "Spinning",
}

// StrongSet is one row of a Strong export that carries a per-set time.
type StrongSet struct {
	// Exercise is Strong's name for the exercise, before aliasing.
	Exercise string
	// Order is Strong's 1-based Set Order within the exercise.
	Order int
	// Seconds is the per-set time Strong recorded. This is the field the import lost.
	Seconds int
}

// StrongWorkout is one workout from a Strong export, keyed by its local start time.
type StrongWorkout struct {
	Start time.Time
	Timed map[string][]StrongSet
	Name  string
}

// ParseStrong reads a Strong CSV export and returns the workouts that contain at least one timed
// set. Rows for untimed exercises are dropped: they carry nothing the repair needs.
func ParseStrong(r io.Reader) ([]StrongWorkout, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading Strong CSV header: %w", err)
	}

	column := map[string]int{}
	for i, name := range header {
		column[strings.TrimSpace(name)] = i
	}

	for _, required := range []string{"Date", "Workout Name", "Exercise Name", "Set Order", "Seconds"} {
		if _, ok := column[required]; !ok {
			return nil, fmt.Errorf("Strong CSV is missing the %q column", required)
		}
	}

	var (
		order    []time.Time
		workouts = map[time.Time]*StrongWorkout{}
	)

	for {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("reading Strong CSV: %w", readErr)
		}

		seconds, convErr := strconv.Atoi(strings.TrimSpace(record[column["Seconds"]]))
		if convErr != nil || seconds <= 0 {
			continue
		}

		start, timeErr := time.ParseInLocation(time.DateTime, record[column["Date"]], time.UTC)
		if timeErr != nil {
			return nil, fmt.Errorf("parsing Strong date %q: %w", record[column["Date"]], timeErr)
		}

		setOrder, convErr := strconv.Atoi(strings.TrimSpace(record[column["Set Order"]]))
		if convErr != nil {
			return nil, fmt.Errorf("parsing Strong set order %q: %w", record[column["Set Order"]], convErr)
		}

		w, ok := workouts[start]
		if !ok {
			w = &StrongWorkout{Start: start, Name: record[column["Workout Name"]], Timed: map[string][]StrongSet{}}
			workouts[start] = w
			order = append(order, start)
		}

		exercise := record[column["Exercise Name"]]
		w.Timed[exercise] = append(w.Timed[exercise], StrongSet{Exercise: exercise, Order: setOrder, Seconds: seconds})
	}

	out := make([]StrongWorkout, 0, len(order))
	for _, start := range order {
		w := workouts[start]
		for name := range w.Timed {
			sets := w.Timed[name]
			sort.Slice(sets, func(i, j int) bool { return sets[i].Order < sets[j].Order })
		}
		out = append(out, *w)
	}

	return out, nil
}

// ErrNoOffset means no single whole-hour offset lines every Strong workout up with a Hevy workout.
var ErrNoOffset = errors.New("no consistent UTC offset aligns the Strong export with the Hevy history")

// DetectOffset finds the fixed offset between the export's local timestamps and Hevy's UTC start
// times.
//
// Strong writes no timezone, so the offset has to be recovered rather than assumed. It is derived
// rather than configured because the answer is checkable: the correct offset lines up every single
// workout to the second, and a wrong one lines up almost none. DetectOffset requires the winning
// offset to match every workout, so a partial coincidence cannot be mistaken for the answer.
func DetectOffset(strong []StrongWorkout, workouts []hevy.Workout) (time.Duration, error) {
	starts := map[time.Time]bool{}
	for i := range workouts {
		starts[workouts[i].StartTime.UTC()] = true
	}

	for hours := -14; hours <= 14; hours++ {
		offset := time.Duration(hours) * time.Hour

		matched := 0
		for i := range strong {
			if starts[strong[i].Start.Add(offset)] {
				matched++
			}
		}

		if matched == len(strong) && matched > 0 {
			return offset, nil
		}
	}

	return 0, ErrNoOffset
}

// Restoration is one exercise's worth of recovered per-set times.
type Restoration struct {
	WorkoutID string
	Title     string
	Exercise  string
	Start     time.Time
	// Seconds is indexed by set, in set order.
	Seconds []int
	// ExerciseIndex identifies the exercise within the workout.
	ExerciseIndex int
}

// RestorePlan is the result of matching a Strong export against the Hevy history.
type RestorePlan struct {
	// Restorations are the exercises whose times can be recovered exactly.
	Restorations []Restoration
	// Unmatched describes timed Strong exercises that could not be resolved to exactly one Hevy
	// exercise with the same number of sets. These are reported rather than guessed at.
	Unmatched []string
	// Offset is the timezone offset DetectOffset recovered.
	Offset time.Duration
}

// Sets counts the sets the plan would write.
func (p *RestorePlan) Sets() int {
	var n int
	for i := range p.Restorations {
		n += len(p.Restorations[i].Seconds)
	}
	return n
}

// BuildRestorePlan matches every timed Strong exercise to the Hevy exercise it became.
//
// Matching is by identity, never by position: the import reordered the exercises within each
// workout, so the nth exercise in the export is not the nth exercise in Hevy. A match must resolve
// to exactly one exercise of the aliased name carrying exactly as many sets as the export did.
// Anything ambiguous lands in Unmatched.
func BuildRestorePlan(strong []StrongWorkout, workouts []hevy.Workout) (*RestorePlan, error) {
	offset, err := DetectOffset(strong, workouts)
	if err != nil {
		return nil, err
	}

	byStart := map[time.Time]*hevy.Workout{}
	for i := range workouts {
		byStart[workouts[i].StartTime.UTC()] = &workouts[i]
	}

	plan := &RestorePlan{Offset: offset}

	for i := range strong {
		sw := &strong[i]

		w, ok := byStart[sw.Start.Add(offset)]
		if !ok {
			plan.Unmatched = append(plan.Unmatched, fmt.Sprintf("%s %q: no Hevy workout at that time", sw.Start.Format(time.DateTime), sw.Name))
			continue
		}

		names := make([]string, 0, len(sw.Timed))
		for name := range sw.Timed {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			sets := sw.Timed[name]

			target, aliased := strongAliases[name]
			if !aliased {
				plan.Unmatched = append(plan.Unmatched, fmt.Sprintf("%s %q: no alias for Strong exercise %q", sw.Start.Format(time.DateOnly), w.Title, name))
				continue
			}

			var matches []*hevy.WorkoutExercise
			for j := range w.Exercises {
				if w.Exercises[j].Title == target {
					matches = append(matches, &w.Exercises[j])
				}
			}

			if len(matches) != 1 {
				plan.Unmatched = append(plan.Unmatched, fmt.Sprintf("%s %q: %q matched %d Hevy exercises", sw.Start.Format(time.DateOnly), w.Title, target, len(matches)))
				continue
			}

			if len(matches[0].Sets) != len(sets) {
				plan.Unmatched = append(plan.Unmatched, fmt.Sprintf("%s %q: %q has %d sets in Strong but %d in Hevy", sw.Start.Format(time.DateOnly), w.Title, target, len(sets), len(matches[0].Sets)))
				continue
			}

			seconds := make([]int, len(sets))
			for k := range sets {
				seconds[k] = sets[k].Seconds
			}

			plan.Restorations = append(plan.Restorations, Restoration{
				WorkoutID:     w.ID,
				Title:         w.Title,
				Start:         w.StartTime,
				Exercise:      target,
				ExerciseIndex: matches[0].Index,
				Seconds:       seconds,
			})
		}
	}

	sort.Slice(plan.Restorations, func(i, j int) bool {
		return plan.Restorations[i].Start.Before(plan.Restorations[j].Start)
	})

	return plan, nil
}

// ApplyRestorations builds the update that writes the recovered times back into w.
func ApplyRestorations(w *hevy.Workout, restorations []Restoration) (*hevy.WorkoutRequest, error) {
	seconds := map[int][]int{}
	for i := range restorations {
		seconds[restorations[i].ExerciseIndex] = restorations[i].Seconds
	}

	// Deliberately not Repair: a restore must leave every duration it is not restoring exactly as
	// it found it, including legitimate ones the import never touched.
	req, err := buildRequest(w)
	if err != nil {
		return nil, err
	}

	for i := range w.Exercises {
		recovered, ok := seconds[w.Exercises[i].Index]
		if !ok {
			continue
		}
		if len(recovered) != len(req.Exercises[i].Sets) {
			return nil, fmt.Errorf("workout %s exercise %d: %d recovered times for %d sets",
				w.ID, w.Exercises[i].Index, len(recovered), len(req.Exercises[i].Sets))
		}
		for j := range recovered {
			req.Exercises[i].Sets[j].DurationSeconds = &recovered[j]
		}
	}

	return req, nil
}
