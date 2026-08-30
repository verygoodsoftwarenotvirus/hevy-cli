package strongimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindGaps(t *testing.T) {
	t.Parallel()

	t.Run("reports blanks only on exercises Hevy times", func(t *testing.T) {
		t.Parallel()

		w := hevyWorkout()

		gaps := FindGaps(w, testLookup)
		require.Len(t, gaps, 1, "the untimed squat has no duration to be missing")
		assert.Equal(t, "Plank", gaps[0].Exercise)
		assert.Equal(t, 2, gaps[0].Blank)
	})

	t.Run("a fully timed exercise is not a gap", func(t *testing.T) {
		t.Parallel()

		w := hevyWorkout()
		w[0].Exercises[0].Sets[0].DurationSeconds = ptr(70)
		w[0].Exercises[0].Sets[1].DurationSeconds = ptr(45)

		assert.Empty(t, FindGaps(w, testLookup))
	})
}

func TestApplyFills(t *testing.T) {
	t.Parallel()

	t.Run("fills blanks and notes that the value is an estimate", func(t *testing.T) {
		t.Parallel()

		w := &hevyWorkout()[0]
		w.Exercises[0].Notes = " "

		req, filled, err := ApplyFills(w, map[string]int{"Plank": 65}, DefaultFillNote)
		require.NoError(t, err)
		assert.Equal(t, 2, filled)
		assert.Equal(t, 65, *req.Exercises[0].Sets[0].DurationSeconds)
		assert.Equal(t, 65, *req.Exercises[0].Sets[1].DurationSeconds)
		assert.Equal(t, DefaultFillNote, req.Exercises[0].Notes)
	})

	// Running a fill twice must not drag a measured value toward the estimate.
	t.Run("never overwrites a set that already has a duration", func(t *testing.T) {
		t.Parallel()

		w := &hevyWorkout()[0]
		w.Exercises[0].Sets[0].DurationSeconds = ptr(70)

		req, filled, err := ApplyFills(w, map[string]int{"Plank": 65}, "")
		require.NoError(t, err)
		assert.Equal(t, 1, filled)
		assert.Equal(t, 70, *req.Exercises[0].Sets[0].DurationSeconds, "the real value must survive")
		assert.Equal(t, 65, *req.Exercises[0].Sets[1].DurationSeconds)
	})

	t.Run("does not repeat the note on a second pass", func(t *testing.T) {
		t.Parallel()

		w := &hevyWorkout()[0]
		w.Exercises[0].Notes = DefaultFillNote

		req, _, err := ApplyFills(w, map[string]int{"Plank": 65}, DefaultFillNote)
		require.NoError(t, err)
		assert.Equal(t, DefaultFillNote, req.Exercises[0].Notes)
	})

	t.Run("preserves an existing human note", func(t *testing.T) {
		t.Parallel()

		w := &hevyWorkout()[0]
		w.Exercises[0].Notes = "Seat setting 3"

		req, _, err := ApplyFills(w, map[string]int{"Plank": 65}, DefaultFillNote)
		require.NoError(t, err)
		assert.Contains(t, req.Exercises[0].Notes, "Seat setting 3")
		assert.Contains(t, req.Exercises[0].Notes, DefaultFillNote)
	})

	t.Run("an exercise with no fill value is untouched", func(t *testing.T) {
		t.Parallel()

		_, _, err := ApplyFills(&hevyWorkout()[0], map[string]int{"Side Plank": 30}, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nothing to fill")
	})
}
