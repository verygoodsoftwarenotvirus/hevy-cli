package strongimport

import (
	"strings"
	"testing"
	"time"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const strongCSV = `Date,Workout Name,Duration,Exercise Name,Set Order,Weight,Reps,Distance,Seconds,Notes,Workout Notes,RPE
2024-10-16 05:59:03,"Legs/Abs 1 (WFH)",215m,"Squat (Barbell)",1,195.0,5,0,0,"","",
2024-10-16 05:59:03,"Legs/Abs 1 (WFH)",215m,"Plank",1,0,0,0,70,"","",
2024-10-16 05:59:03,"Legs/Abs 1 (WFH)",215m,"Plank",2,0,0,0,45,"","",
`

// hevyWorkout mirrors the CSV above, with the exercises in a different order: the import reordered
// them, so a positional join would silently write the plank times onto the squat.
func hevyWorkout() []hevy.Workout {
	start := time.Date(2024, time.October, 16, 11, 59, 3, 0, time.UTC)

	return []hevy.Workout{{
		ID:        "workout-1",
		Title:     "Legs/Abs 1 (WFH)",
		StartTime: start,
		EndTime:   start.Add(defaultStampSecs * time.Second),
		Exercises: []hevy.WorkoutExercise{
			{Index: 0, Title: "Plank", ExerciseTemplateID: plankTemplate, Sets: []hevy.WorkoutSet{
				{Index: 0, Type: hevy.SetTypeNormal}, {Index: 1, Type: hevy.SetTypeNormal},
			}},
			{Index: 1, Title: "Squat (Barbell)", ExerciseTemplateID: squatTemplate, Sets: []hevy.WorkoutSet{
				{Index: 0, Type: hevy.SetTypeNormal},
			}},
		},
	}}
}

func TestParseStrong(t *testing.T) {
	t.Parallel()

	t.Run("keeps only timed sets", func(t *testing.T) {
		t.Parallel()

		workouts, err := ParseStrong(strings.NewReader(strongCSV))
		require.NoError(t, err)
		require.Len(t, workouts, 1)

		assert.Equal(t, "Legs/Abs 1 (WFH)", workouts[0].Name)
		assert.Len(t, workouts[0].Timed, 1, "the untimed squat should not appear")
		assert.Equal(t, []StrongSet{
			{Exercise: "Plank", Order: 1, Seconds: 70},
			{Exercise: "Plank", Order: 2, Seconds: 45},
		}, workouts[0].Timed["Plank"])
	})

	t.Run("rejects a file missing a needed column", func(t *testing.T) {
		t.Parallel()

		_, err := ParseStrong(strings.NewReader(
			"Date,Workout Name,Exercise Name,Set Order\n2024-01-01 00:00:00,Legs,Plank,1\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Seconds")
	})
}

func TestDetectOffset(t *testing.T) {
	t.Parallel()

	t.Run("recovers the exporting device's offset", func(t *testing.T) {
		t.Parallel()

		strong, err := ParseStrong(strings.NewReader(strongCSV))
		require.NoError(t, err)

		offset, err := DetectOffset(strong, hevyWorkout())
		require.NoError(t, err)
		assert.Equal(t, 6*time.Hour, offset)
	})

	t.Run("refuses when nothing lines up", func(t *testing.T) {
		t.Parallel()

		strong, err := ParseStrong(strings.NewReader(strongCSV))
		require.NoError(t, err)

		w := hevyWorkout()
		w[0].StartTime = w[0].StartTime.Add(7 * time.Minute)

		_, err = DetectOffset(strong, w)
		require.ErrorIs(t, err, ErrNoOffset)
	})
}

func TestBuildRestorePlan(t *testing.T) {
	t.Parallel()

	// The regression that matters: the import reordered exercises, so the plan must find the plank
	// at Hevy index 0 even though Strong listed it second.
	t.Run("matches by identity, not position", func(t *testing.T) {
		t.Parallel()

		strong, err := ParseStrong(strings.NewReader(strongCSV))
		require.NoError(t, err)

		plan, err := BuildRestorePlan(strong, hevyWorkout())
		require.NoError(t, err)
		require.Len(t, plan.Restorations, 1)
		assert.Empty(t, plan.Unmatched)

		assert.Equal(t, "Plank", plan.Restorations[0].Exercise)
		assert.Equal(t, 0, plan.Restorations[0].ExerciseIndex)
		assert.Equal(t, []int{70, 45}, plan.Restorations[0].Seconds)
	})

	t.Run("reports rather than guesses when the set counts disagree", func(t *testing.T) {
		t.Parallel()

		strong, err := ParseStrong(strings.NewReader(strongCSV))
		require.NoError(t, err)

		w := hevyWorkout()
		w[0].Exercises[0].Sets = w[0].Exercises[0].Sets[:1]

		plan, err := BuildRestorePlan(strong, w)
		require.NoError(t, err)
		assert.Empty(t, plan.Restorations)
		require.Len(t, plan.Unmatched, 1)
		assert.Contains(t, plan.Unmatched[0], "2 sets in Strong but 1 in Hevy")
	})
}

func TestApplyRestorations(t *testing.T) {
	t.Parallel()

	t.Run("writes recovered times and leaves other durations alone", func(t *testing.T) {
		t.Parallel()

		w := &hevyWorkout()[0]
		w.Exercises[1].Sets[0].DurationSeconds = ptr(999) // a duration the restore must not disturb

		req, err := ApplyRestorations(w, []Restoration{{ExerciseIndex: 0, Seconds: []int{70, 45}}})
		require.NoError(t, err)

		assert.Equal(t, 70, *req.Exercises[0].Sets[0].DurationSeconds)
		assert.Equal(t, 45, *req.Exercises[0].Sets[1].DurationSeconds)
		assert.Equal(t, 999, *req.Exercises[1].Sets[0].DurationSeconds)
	})

	t.Run("refuses a count mismatch", func(t *testing.T) {
		t.Parallel()

		_, err := ApplyRestorations(&hevyWorkout()[0], []Restoration{{ExerciseIndex: 0, Seconds: []int{70}}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "1 recovered times for 2 sets")
	})
}
