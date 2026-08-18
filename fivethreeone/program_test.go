package fivethreeone

import (
	"fmt"
	"testing"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAssistanceScheme(T *testing.T) {
	T.Parallel()

	for input, expected := range map[string]AssistanceScheme{
		"bbb":              AssistanceBBB,
		"BBB":              AssistanceBBB,
		"boring but big":   AssistanceBBB,
		"fsl":              AssistanceFSL,
		"fsl-paused":       AssistanceFSLPaused,
		"Paused FSL":       AssistanceFSLPaused,
		"paused":           AssistanceFSLPaused,
		" FSL ":            AssistanceFSL,
		"first set last":   AssistanceFSL,
		"first-set-last":   AssistanceFSL,
		"none":             AssistanceNone,
		"off":              AssistanceNone,
		"":                 AssistanceNone,
		"unknown-nonsense": "",
	} {
		T.Run(input, func(t *testing.T) {
			t.Parallel()

			actual, ok := ParseAssistanceScheme(input)
			assert.Equal(t, expected != "", ok)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestCalculateAssistanceSets(T *testing.T) {
	T.Parallel()

	const trainingMax = 100.0

	T.Run("BBB is 5x10 at half the training max", func(t *testing.T) {
		t.Parallel()

		sets := CalculateAssistanceSets(Squat, AssistanceBBB, trainingMax, 2, false)
		require.Len(t, sets, 5)
		for _, s := range sets {
			assert.InDelta(t, 50.0, s.WeightKg, 0.001)
			assert.Equal(t, 10, s.Reps)
		}
	})

	T.Run("no assistance on the deload week", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, CalculateAssistanceSets(Squat, AssistanceFSL, trainingMax, DeloadWeek, false))
		assert.Empty(t, CalculateAssistanceSets(Squat, AssistanceFSLPaused, trainingMax, DeloadWeek, false))
		assert.Empty(t, CalculateAssistanceSets(Squat, AssistanceBBB, trainingMax, DeloadWeek, false))
	})

	T.Run("none and unknown weeks yield nothing", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, CalculateAssistanceSets(Squat, AssistanceNone, trainingMax, 1, false))
		assert.Empty(t, CalculateAssistanceSets(Squat, AssistanceFSL, trainingMax, 9, false))
	})
}

func TestConfigAssistanceScheme(T *testing.T) {
	T.Parallel()

	T.Run("defaults to BBB when unset", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{}
		assert.Equal(t, DefaultAssistanceScheme, cfg.AssistanceScheme())
	})

	T.Run("honors an explicit preset", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{Assistance: AssistanceFSL}
		assert.Equal(t, AssistanceFSL, cfg.AssistanceScheme())
	})
}

// testTrainingMaxes are the training maxes the golden main-set expectations below were
// captured from.
var testTrainingMaxes = map[Lift]float64{
	Squat:         90.0,
	BenchPress:    67.5,
	OverheadPress: 47.5,
	Deadlift:      142.5,
}

// TestCalculateRoutineSets_Golden pins the main lift's warmup and working weights for
// every lift and week. The supplemental presets are layered on top of these sets and must
// never perturb them: any diff here means main set generation changed.
func TestCalculateRoutineSets_Golden(T *testing.T) {
	T.Parallel()

	// Indexed rather than ranged by value: the table's rows are wide enough that
	// gocritic's rangeValCopy objects to copying each one.
	table := []struct {
		lift    Lift
		warmups []float64
		working []float64
		week    int
	}{
		{lift: Squat, week: 1, warmups: []float64{35, 45, 55}, working: []float64{57.5, 67.5, 77.5}},
		{lift: Squat, week: 2, warmups: []float64{35, 45, 55}, working: []float64{62.5, 72.5, 80}},
		{lift: Squat, week: 3, warmups: []float64{35, 45, 55}, working: []float64{67.5, 77.5, 85}},
		{lift: BenchPress, week: 1, warmups: []float64{27.5, 35, 40}, working: []float64{45, 50, 57.5}},
		{lift: BenchPress, week: 2, warmups: []float64{27.5, 35, 40}, working: []float64{47.5, 55, 60}},
		{lift: BenchPress, week: 3, warmups: []float64{27.5, 35, 40}, working: []float64{50, 57.5, 65}},
		{lift: OverheadPress, week: 1, warmups: []float64{20, 25, 27.5}, working: []float64{30, 35, 40}},
		{lift: OverheadPress, week: 2, warmups: []float64{20, 25, 27.5}, working: []float64{32.5, 37.5, 42.5}},
		{lift: OverheadPress, week: 3, warmups: []float64{20, 25, 27.5}, working: []float64{35, 40, 45}},
		{lift: Deadlift, week: 1, warmups: []float64{57.5, 72.5, 85}, working: []float64{92.5, 107.5, 120}},
		{lift: Deadlift, week: 2, warmups: []float64{57.5, 72.5, 85}, working: []float64{100, 115, 127.5}},
		{lift: Deadlift, week: 3, warmups: []float64{57.5, 72.5, 85}, working: []float64{107.5, 120, 135}},
	}

	for i := range table {
		tc := &table[i]
		T.Run(fmt.Sprintf("%s week %d", tc.lift, tc.week), func(t *testing.T) {
			t.Parallel()

			sets := CalculateRoutineSets(testTrainingMaxes[tc.lift], tc.week, false)
			require.Len(t, sets, len(tc.warmups)+len(tc.working))

			for i, expected := range tc.warmups {
				assert.Equal(t, hevy.SetTypeWarmup, sets[i].Type)
				assert.InDelta(t, expected, sets[i].WeightKg, 0.001)
			}
			for i, expected := range tc.working {
				actual := sets[len(tc.warmups)+i]
				assert.Equal(t, hevy.SetTypeNormal, actual.Type)
				assert.InDelta(t, expected, actual.WeightKg, 0.001)
			}
		})
	}
}

// firstWorkingSet returns the week's opening working set — the weight FSL matches.
func firstWorkingSet(t *testing.T, trainingMaxKg float64, week int) CalculatedSet {
	t.Helper()

	sets := CalculateRoutineSets(trainingMaxKg, week, false)
	for i := range sets {
		if sets[i].Type == hevy.SetTypeNormal {
			return sets[i]
		}
	}
	t.Fatalf("no working set in week %d", week)
	return CalculatedSet{}
}

func TestCalculateAssistanceSets_FSL(T *testing.T) {
	T.Parallel()

	// Set counts are per-lift: the deadlift's FSL is capped below the others'.
	expectedSetCount := map[Lift]int{Squat: 5, BenchPress: 5, OverheadPress: 5, Deadlift: 3}

	for lift, trainingMax := range testTrainingMaxes {
		for week := 1; week < DeloadWeek; week++ {
			T.Run(fmt.Sprintf("%s week %d", lift, week), func(t *testing.T) {
				t.Parallel()

				sets := CalculateAssistanceSets(lift, AssistanceFSL, trainingMax, week, false)
				require.Len(t, sets, expectedSetCount[lift])

				// The FSL weight is the week's first working set — compared against the
				// main-set calculation itself so the two can't quietly drift apart.
				want := firstWorkingSet(t, trainingMax, week).WeightKg
				for _, s := range sets {
					assert.InDelta(t, want, s.WeightKg, 0.001)
					// Reps stay at 5 whatever the week's main rep scheme is.
					assert.Equal(t, 5, s.Reps)
					assert.Equal(t, hevy.SetTypeNormal, s.Type)
					assert.False(t, s.IsAMRAP)
				}
			})
		}
	}
}

func TestCalculateAssistanceSets_PausedFSL(T *testing.T) {
	T.Parallel()

	T.Run("squat paused weight is 90% of the FSL weight", func(t *testing.T) {
		t.Parallel()

		// Squat TM 90: FSL is 57.5 / 62.5 / 67.5, and 90% of each rounds to the nearest
		// 2.5 kg. Week 2's 56.25 sits exactly between 55 and 57.5, so it rounds up.
		for week, expected := range map[int]float64{1: 52.5, 2: 57.5, 3: 60} {
			sets := CalculateAssistanceSets(Squat, AssistanceFSLPaused, testTrainingMaxes[Squat], week, false)
			require.Len(t, sets, 5)

			fsl := firstWorkingSet(t, testTrainingMaxes[Squat], week).WeightKg
			assert.InDelta(t, RoundWeight(fsl*0.9), sets[0].WeightKg, 0.001)
			for _, s := range sets {
				assert.InDelta(t, expected, s.WeightKg, 0.001, "week %d", week)
				assert.Equal(t, 5, s.Reps)
			}
		}
	})

	T.Run("the pause is surfaced as a cue", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "2s pause at the bottom", AssistanceFSLPaused.Cue())
		assert.Empty(t, AssistanceFSL.Cue())
		assert.Empty(t, AssistanceBBB.Cue())
		assert.Contains(t, AssistanceFSLPaused.DisplayNameFor(Squat), "2s pause at the bottom")
		assert.True(t, AssistanceFSLPaused.InMainExercise())
	})

	T.Run("the deadlift cap applies to the paused variant too", func(t *testing.T) {
		t.Parallel()

		assert.Len(t, CalculateAssistanceSets(Deadlift, AssistanceFSLPaused, testTrainingMaxes[Deadlift], 1, false), 3)
	})
}

func TestConfigAssistanceSchemeFor(T *testing.T) {
	T.Parallel()

	cfg := &Config{
		Assistance: AssistanceFSL,
		Lifts: map[Lift]LiftConfig{
			Squat:    {Assistance: AssistanceFSLPaused},
			Deadlift: {},
		},
	}

	T.Run("a lift override wins", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, AssistanceFSLPaused, cfg.AssistanceSchemeFor(Squat))
	})

	T.Run("lifts without an override follow the program", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, AssistanceFSL, cfg.AssistanceSchemeFor(Deadlift))
		assert.Equal(t, AssistanceFSL, cfg.AssistanceSchemeFor(BenchPress))
	})

	T.Run("clearing an override restores the program preset", func(t *testing.T) {
		t.Parallel()

		cleared := &Config{Assistance: AssistanceFSL, Lifts: map[Lift]LiftConfig{Squat: {}}}
		assert.Equal(t, AssistanceFSL, cleared.AssistanceSchemeFor(Squat))
	})
}
