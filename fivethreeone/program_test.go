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

// cycle7TrainingMaxes are the Cycle 7 training maxes (bench is disabled that cycle).
var cycle7TrainingMaxes = map[Lift]float64{
	Squat:         95.0,
	OverheadPress: 50.0,
	Deadlift:      135.0,
}

func TestCalculateRoutineSets_Cycle7TopSets(T *testing.T) {
	T.Parallel()

	table := []struct {
		lift Lift
		top  float64
		week int
	}{
		{lift: Squat, week: 1, top: 80},
		{lift: Squat, week: 2, top: 85},
		{lift: Squat, week: 3, top: 90},
		{lift: Deadlift, week: 1, top: 115},
		{lift: Deadlift, week: 2, top: 122.5},
		{lift: Deadlift, week: 3, top: 127.5},
		{lift: OverheadPress, week: 1, top: 42.5},
		{lift: OverheadPress, week: 2, top: 45},
		{lift: OverheadPress, week: 3, top: 47.5},
	}

	for _, tc := range table {
		T.Run(fmt.Sprintf("%s week %d", tc.lift, tc.week), func(t *testing.T) {
			t.Parallel()

			sets := CalculateRoutineSets(cycle7TrainingMaxes[tc.lift], tc.week, false)
			require.NotEmpty(t, sets)

			top := sets[len(sets)-1]
			assert.True(t, top.IsAMRAP)
			assert.Equal(t, tc.top, top.WeightKg)
		})
	}
}

// TestCalculateRoutineSets_Cycle7Golden pins every main-lift set for the Cycle 7 training
// maxes, exactly as the code produced them before custom days, disabled lifts, and set
// annotations were added. The comparison is exact (no delta): those changes must not
// move a single main set.
func TestCalculateRoutineSets_Cycle7Golden(T *testing.T) {
	T.Parallel()

	w := func(kg float64, reps int) CalculatedSet {
		return CalculatedSet{Type: hevy.SetTypeWarmup, WeightKg: kg, Reps: reps}
	}
	n := func(kg float64, reps int) CalculatedSet {
		return CalculatedSet{Type: hevy.SetTypeNormal, WeightKg: kg, Reps: reps}
	}
	amrap := func(kg float64, reps int) CalculatedSet {
		return CalculatedSet{Type: hevy.SetTypeNormal, WeightKg: kg, Reps: reps, IsAMRAP: true}
	}

	// Indexed rather than ranged by value: see TestCalculateRoutineSets_Golden.
	table := []struct {
		lift     Lift
		expected []CalculatedSet
		week     int
	}{
		{lift: Squat, week: 1, expected: []CalculatedSet{w(37.5, 5), w(47.5, 5), w(57.5, 3), n(62.5, 5), n(72.5, 5), amrap(80, 5)}},
		{lift: Squat, week: 2, expected: []CalculatedSet{w(37.5, 5), w(47.5, 5), w(57.5, 3), n(67.5, 3), n(75, 3), amrap(85, 3)}},
		{lift: Squat, week: 3, expected: []CalculatedSet{w(37.5, 5), w(47.5, 5), w(57.5, 3), n(72.5, 5), n(80, 3), amrap(90, 1)}},
		{lift: Squat, week: 4, expected: []CalculatedSet{n(37.5, 5), n(47.5, 5), n(57.5, 5)}},
		{lift: Deadlift, week: 1, expected: []CalculatedSet{w(55, 5), w(67.5, 5), w(80, 3), n(87.5, 5), n(102.5, 5), amrap(115, 5)}},
		{lift: Deadlift, week: 2, expected: []CalculatedSet{w(55, 5), w(67.5, 5), w(80, 3), n(95, 3), n(107.5, 3), amrap(122.5, 3)}},
		{lift: Deadlift, week: 3, expected: []CalculatedSet{w(55, 5), w(67.5, 5), w(80, 3), n(102.5, 5), n(115, 3), amrap(127.5, 1)}},
		{lift: Deadlift, week: 4, expected: []CalculatedSet{n(55, 5), n(67.5, 5), n(80, 5)}},
		{lift: OverheadPress, week: 1, expected: []CalculatedSet{w(20, 5), w(25, 5), w(30, 3), n(32.5, 5), n(37.5, 5), amrap(42.5, 5)}},
		{lift: OverheadPress, week: 2, expected: []CalculatedSet{w(20, 5), w(25, 5), w(30, 3), n(35, 3), n(40, 3), amrap(45, 3)}},
		{lift: OverheadPress, week: 3, expected: []CalculatedSet{w(20, 5), w(25, 5), w(30, 3), n(37.5, 5), n(42.5, 3), amrap(47.5, 1)}},
		{lift: OverheadPress, week: 4, expected: []CalculatedSet{n(20, 5), n(25, 5), n(30, 5)}},
	}

	for i := range table {
		tc := &table[i]
		T.Run(fmt.Sprintf("%s week %d", tc.lift, tc.week), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, CalculateRoutineSets(cycle7TrainingMaxes[tc.lift], tc.week, false))
		})
	}
}

func TestCalculateAssistanceSets_Cycle7(T *testing.T) {
	T.Parallel()

	table := []struct {
		lift     Lift
		scheme   AssistanceScheme
		weightKg float64
		sets     int
		week     int
	}{
		// The deadlift's FSL stays capped at 3 sets.
		{lift: Deadlift, scheme: AssistanceFSL, week: 1, sets: 3, weightKg: 87.5},
		{lift: Deadlift, scheme: AssistanceFSL, week: 2, sets: 3, weightKg: 95},
		{lift: Deadlift, scheme: AssistanceFSL, week: 3, sets: 3, weightKg: 102.5},
		{lift: OverheadPress, scheme: AssistanceFSL, week: 1, sets: 5, weightKg: 32.5},
		{lift: OverheadPress, scheme: AssistanceFSL, week: 2, sets: 5, weightKg: 35},
		{lift: OverheadPress, scheme: AssistanceFSL, week: 3, sets: 5, weightKg: 37.5},
		// Squat keeps its paused-FSL override. Week 1's 56.25 (90% of 62.5) is a
		// rounding tie and goes up to 57.5.
		{lift: Squat, scheme: AssistanceFSLPaused, week: 1, sets: 5, weightKg: 57.5},
		{lift: Squat, scheme: AssistanceFSLPaused, week: 2, sets: 5, weightKg: 60},
		{lift: Squat, scheme: AssistanceFSLPaused, week: 3, sets: 5, weightKg: 65},
	}

	for _, tc := range table {
		T.Run(fmt.Sprintf("%s %s week %d", tc.lift, tc.scheme, tc.week), func(t *testing.T) {
			t.Parallel()

			sets := CalculateAssistanceSets(tc.lift, tc.scheme, cycle7TrainingMaxes[tc.lift], tc.week, false)
			require.Len(t, sets, tc.sets)
			for _, s := range sets {
				assert.Equal(t, CalculatedSet{Type: hevy.SetTypeNormal, WeightKg: tc.weightKg, Reps: 5}, s)
			}
		})
	}
}

func TestCalculateAssistanceSetsWithCount(T *testing.T) {
	T.Parallel()

	const trainingMax = 100.0

	for name, tc := range map[string]struct {
		scheme   AssistanceScheme
		week     int
		fslSets  int
		sets     int
		weightKg float64
		reps     int
	}{
		"fsl uses the given count":            {scheme: AssistanceFSL, week: 2, fslSets: 4, sets: 4, weightKg: 70, reps: 5},
		"fsl taper bottoms out":               {scheme: AssistanceFSL, week: 3, fslSets: 2, sets: 2, weightKg: 75, reps: 5},
		"paused fsl uses the given count":     {scheme: AssistanceFSLPaused, week: 3, fslSets: 3, sets: 3, weightKg: 67.5, reps: 5},
		"bbb ignores the fsl count":           {scheme: AssistanceBBB, week: 1, fslSets: 2, sets: 5, weightKg: 50, reps: 10},
		"no fsl on the deload whatever count": {scheme: AssistanceFSL, week: DeloadWeek, fslSets: 5},
		"a zero count prescribes nothing":     {scheme: AssistanceFSL, week: 1, fslSets: 0},
		"a negative count prescribes nothing": {scheme: AssistanceFSL, week: 1, fslSets: -1},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			sets := CalculateAssistanceSetsWithCount(tc.scheme, trainingMax, tc.week, tc.fslSets, false)
			require.Len(t, sets, tc.sets)
			for _, s := range sets {
				assert.Equal(t, CalculatedSet{Type: hevy.SetTypeNormal, WeightKg: tc.weightKg, Reps: tc.reps}, s)
			}
		})
	}

	T.Run("the per-lift form matches the lift's default count", func(t *testing.T) {
		t.Parallel()

		for _, lift := range AllLifts() {
			assert.Equal(t,
				CalculateAssistanceSetsWithCount(AssistanceFSL, trainingMax, 1, lift.FSLSetCount(), false),
				CalculateAssistanceSets(lift, AssistanceFSL, trainingMax, 1, false),
				lift)
		}
	})
}

func TestAssistanceSchemeDisplayNameForSets(T *testing.T) {
	T.Parallel()

	for name, tc := range map[string]struct {
		scheme   AssistanceScheme
		expected string
		fslSets  int
	}{
		"fsl":                  {scheme: AssistanceFSL, fslSets: 4, expected: "First Set Last (4×5 @ first working set)"},
		"paused fsl":           {scheme: AssistanceFSLPaused, fslSets: 3, expected: "Paused First Set Last (3×5 @ 90% of FSL, 2s pause at the bottom)"},
		"bbb ignores count":    {scheme: AssistanceBBB, fslSets: 2, expected: "Boring But Big (5×10 @ 50% TM)"},
		"none ignores count":   {scheme: AssistanceNone, fslSets: 2, expected: "None"},
		"fsl default (5 sets)": {scheme: AssistanceFSL, fslSets: Squat.FSLSetCount(), expected: AssistanceFSL.DisplayNameFor(Squat)},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, tc.scheme.DisplayNameForSets(tc.fslSets))
		})
	}
}
