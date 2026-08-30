package strongimport

import (
	"testing"
	"time"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	squatTemplate    = "D04AC939"
	plankTemplate    = "C6C9B8A0"
	ellipticalTmpl   = "E11117CA"
	unknownTemplate  = "00000000"
	defaultStampSecs = 12900
)

func testLookup(id string) (hevy.ExerciseType, bool) {
	switch id {
	case squatTemplate:
		return hevy.ExerciseTypeWeightReps, true
	case plankTemplate:
		return hevy.ExerciseTypeDuration, true
	case ellipticalTmpl:
		return hevy.ExerciseTypeDistanceDuration, true
	default:
		return "", false
	}
}

//go:fix inline
func ptr[T any](v T) *T { return new(v) }

// workout builds a workout of the given elapsed length from (templateID, per-set durations) pairs.
func workout(elapsed int, exercises ...exerciseSpec) *hevy.Workout {
	start := time.Date(2024, time.October, 16, 11, 59, 3, 0, time.UTC)

	w := &hevy.Workout{
		ID:        "workout-1",
		Title:     "Legs/Abs 1 (WFH)",
		StartTime: start,
		EndTime:   start.Add(time.Duration(elapsed) * time.Second),
	}

	for i, spec := range exercises {
		e := hevy.WorkoutExercise{
			Index:              i,
			Title:              spec.title,
			ExerciseTemplateID: spec.template,
		}
		for j, d := range spec.durations {
			e.Sets = append(e.Sets, hevy.WorkoutSet{Index: j, Type: hevy.SetTypeNormal, DurationSeconds: d})
		}
		w.Exercises = append(w.Exercises, e)
	}

	return w
}

type exerciseSpec struct {
	template  string
	title     string
	durations []*int
}

func stamped(n int) []*int {
	out := make([]*int, n)
	for i := range out {
		out[i] = new(defaultStampSecs)
	}
	return out
}

func TestDetect(t *testing.T) {
	t.Parallel()

	t.Run("the imported signature", func(t *testing.T) {
		t.Parallel()

		w := workout(defaultStampSecs,
			exerciseSpec{squatTemplate, "Squat (Barbell)", stamped(7)},
			exerciseSpec{plankTemplate, "Plank", stamped(3)},
		)

		finding, ok := Detect(w, testLookup)
		require.True(t, ok)
		assert.Equal(t, "workout-1", finding.WorkoutID)
		assert.Equal(t, defaultStampSecs, finding.StampedSeconds)
		assert.Equal(t, 10, finding.Sets)
		assert.Equal(t, []string{"Plank"}, finding.VisibleExercises)
	})

	t.Run("damage Hevy never displays is still reported", func(t *testing.T) {
		t.Parallel()

		w := workout(defaultStampSecs, exerciseSpec{squatTemplate, "Squat (Barbell)", stamped(7)})

		finding, ok := Detect(w, testLookup)
		require.True(t, ok)
		assert.Empty(t, finding.VisibleExercises)
		assert.Equal(t, 7, finding.Sets)
	})

	t.Run("a healthy workout has no set durations at all", func(t *testing.T) {
		t.Parallel()

		w := workout(defaultStampSecs,
			exerciseSpec{squatTemplate, "Squat (Barbell)", []*int{nil, nil, nil}},
		)

		_, ok := Detect(w, testLookup)
		assert.False(t, ok)
	})

	t.Run("a genuinely timed plank is left alone", func(t *testing.T) {
		t.Parallel()

		w := workout(3600,
			exerciseSpec{squatTemplate, "Squat (Barbell)", []*int{nil, nil}},
			exerciseSpec{plankTemplate, "Plank", []*int{new(60), new(75), new(90)}},
		)

		_, ok := Detect(w, testLookup)
		assert.False(t, ok)
	})

	// The case the corroboration clause exists for: one cardio set really can span its whole
	// session, which satisfies the "every duration equals the elapsed time" test by itself.
	t.Run("a lone cardio set spanning its whole session is left alone", func(t *testing.T) {
		t.Parallel()

		w := workout(1800, exerciseSpec{ellipticalTmpl, "Elliptical Trainer", []*int{new(1800)}})

		_, ok := Detect(w, testLookup)
		assert.False(t, ok)
	})

	t.Run("one set differing from the elapsed time disqualifies the workout", func(t *testing.T) {
		t.Parallel()

		durations := stamped(4)
		durations[2] = new(defaultStampSecs - 1)

		w := workout(defaultStampSecs, exerciseSpec{squatTemplate, "Squat (Barbell)", durations})

		_, ok := Detect(w, testLookup)
		assert.False(t, ok)
	})

	t.Run("a partially cleared workout is not re-matched", func(t *testing.T) {
		t.Parallel()

		durations := stamped(4)
		durations[0] = nil

		w := workout(defaultStampSecs, exerciseSpec{squatTemplate, "Squat (Barbell)", durations})

		_, ok := Detect(w, testLookup)
		assert.False(t, ok)
	})

	t.Run("unknown templates cannot corroborate on their own", func(t *testing.T) {
		t.Parallel()

		w := workout(defaultStampSecs, exerciseSpec{unknownTemplate, "Mystery Lift", stamped(5)})

		_, ok := Detect(w, testLookup)
		assert.False(t, ok)
	})

	t.Run("a zero-length workout is skipped", func(t *testing.T) {
		t.Parallel()

		w := workout(0, exerciseSpec{squatTemplate, "Squat (Barbell)", []*int{new(0), new(0)}})

		_, ok := Detect(w, testLookup)
		assert.False(t, ok)
	})

	t.Run("a workout with no sets is skipped", func(t *testing.T) {
		t.Parallel()

		w := workout(defaultStampSecs, exerciseSpec{squatTemplate, "Squat (Barbell)", nil})

		_, ok := Detect(w, testLookup)
		assert.False(t, ok)
	})
}
