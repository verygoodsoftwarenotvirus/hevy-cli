package fivethreeone

import (
	"fmt"
	"slices"
	"testing"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuxToExerciseRequest(T *testing.T) {
	T.Parallel()

	T.Run("repeats one prescription across the configured set count", func(t *testing.T) {
		t.Parallel()

		req := auxToExerciseRequest(AuxiliaryExercise{
			ExerciseTemplateID: "046E25A2",
			WeightKg:           new(61.0),
			Sets:               4,
			Reps:               10,
		})

		assert.Equal(t, "046E25A2", req.ExerciseTemplateID)
		require.NotNil(t, req.RestSeconds)
		assert.Equal(t, defaultAuxRestSeconds, *req.RestSeconds)
		require.Len(t, req.Sets, 4)
		for _, s := range req.Sets {
			assert.Equal(t, hevy.SetTypeNormal, s.Type)
			require.NotNil(t, s.WeightKg)
			assert.InDelta(t, 61.0, *s.WeightKg, 0.001)
			require.NotNil(t, s.Reps)
			assert.Equal(t, 10, *s.Reps)
		}
	})

	T.Run("a set scheme becomes one routine set per rung", func(t *testing.T) {
		t.Parallel()

		req := auxToExerciseRequest(AuxiliaryExercise{
			ExerciseTemplateID: "046E25A2",
			SetScheme: []AuxiliarySet{
				{WeightKg: new(54.43), Reps: 15},
				{WeightKg: new(63.5), Reps: 12},
				{WeightKg: new(68.04), Reps: 10},
				{WeightKg: new(72.57), Reps: 8},
			},
		})

		require.Len(t, req.Sets, 4)
		for i, expected := range []struct {
			weightKg float64
			reps     int
		}{{54.43, 15}, {63.5, 12}, {68.04, 10}, {72.57, 8}} {
			assert.Equal(t, hevy.SetTypeNormal, req.Sets[i].Type)
			require.NotNil(t, req.Sets[i].WeightKg)
			assert.InDelta(t, expected.weightKg, *req.Sets[i].WeightKg, 0.001)
			require.NotNil(t, req.Sets[i].Reps)
			assert.Equal(t, expected.reps, *req.Sets[i].Reps)
		}
	})

	T.Run("duration sets carry a duration instead of reps", func(t *testing.T) {
		t.Parallel()

		req := auxToExerciseRequest(AuxiliaryExercise{
			ExerciseTemplateID: "B9380898",
			DurationSeconds:    new(45),
			RestSeconds:        new(30),
			Sets:               1,
		})

		require.NotNil(t, req.RestSeconds)
		assert.Equal(t, 30, *req.RestSeconds)
		require.Len(t, req.Sets, 1)
		require.NotNil(t, req.Sets[0].DurationSeconds)
		assert.Equal(t, 45, *req.Sets[0].DurationSeconds)
		assert.Nil(t, req.Sets[0].Reps)
	})

	T.Run("a rep range is sent as a range on every set, with no weight when none is given", func(t *testing.T) {
		t.Parallel()

		req := auxToExerciseRequest(AuxiliaryExercise{
			ExerciseTemplateID: "8c524efb",
			Notes:              "RPE 8, stop if pain",
			Sets:               3,
			Reps:               8,
			RepRangeEnd:        10,
		})

		assert.Equal(t, "RPE 8, stop if pain", req.Notes)
		require.Len(t, req.Sets, 3)
		for _, s := range req.Sets {
			assert.Equal(t, &hevy.RepRange{Start: 8, End: 10}, s.RepRange)
			assert.Nil(t, s.Reps)
			assert.Nil(t, s.WeightKg)
		}
	})
}

const (
	cycle7SquatID    = "squat-id"
	cycle7BenchID    = "bench-id"
	cycle7OHPID      = "ohp-id"
	cycle7DeadliftID = "deadlift-id"
	cycle7PressID    = "standing-chest-press-id"
	cycle7LogCleanID = "log-clean-id"
	cycle7FacePullID = "face-pull-id"
)

// cycle7Config mirrors the Cycle 7 program: bench disabled and replaced by a custom
// press day, paused FSL on the squat, Log Clean after the overhead press's FSL (not on
// the deload), and RPE annotations on the deadlift.
func cycle7Config() *Config {
	return &Config{
		CycleNumber: 7,
		Assistance:  AssistanceFSL,
		Lifts: map[Lift]LiftConfig{
			Squat: {
				ExerciseTemplateID: cycle7SquatID,
				TrainingMaxKg:      95,
				EmptyBarWarmup:     true,
				Assistance:         AssistanceFSLPaused,
			},
			BenchPress: {
				ExerciseTemplateID: cycle7BenchID,
				TrainingMaxKg:      70,
				EmptyBarWarmup:     true,
				Disabled:           true,
			},
			OverheadPress: {
				ExerciseTemplateID: cycle7OHPID,
				TrainingMaxKg:      50,
				EmptyBarWarmup:     true,
				AuxiliaryExercises: []AuxiliaryExercise{
					{
						Name:               "Log Clean",
						ExerciseTemplateID: cycle7LogCleanID,
						Notes:              "technique practice, ~55-65 kg",
						Sets:               4,
						Reps:               1,
						RepRangeEnd:        2,
						SkipDeload:         true,
					},
					{ExerciseTemplateID: cycle7FacePullID, WeightKg: new(31.75), Sets: 3, Reps: 10},
				},
			},
			Deadlift: {
				ExerciseTemplateID: cycle7DeadliftID,
				TrainingMaxKg:      135,
				AMRAPNote:          "stop at RPE 8",
				AssistanceNote:     "stop FSL if any set exceeds RPE 8",
			},
		},
		Days: []Day{
			{Lift: Squat},
			{Lift: BenchPress},
			{Name: "Press", Exercises: []AuxiliaryExercise{{
				Name:               "Standing Chest Press",
				ExerciseTemplateID: cycle7PressID,
				Notes:              "RPE 8, stop if pain",
				Sets:               3,
				Reps:               8,
				RepRangeEnd:        10,
			}}},
			{Lift: OverheadPress},
			{Lift: Deadlift},
		},
	}
}

// planByTitle indexes a plan's routine requests by title.
func planByTitle(plan []PlannedRoutine) map[string]*hevy.RoutineRequest {
	byTitle := make(map[string]*hevy.RoutineRequest, len(plan))
	for _, p := range plan {
		byTitle[p.Request.Title] = p.Request
	}
	return byTitle
}

// exerciseByTemplate returns the request's exercise logged against the given template.
func exerciseByTemplate(t *testing.T, req *hevy.RoutineRequest, templateID string) (int, *hevy.RoutineExerciseRequest) {
	t.Helper()

	for i := range req.Exercises {
		if req.Exercises[i].ExerciseTemplateID == templateID {
			return i, &req.Exercises[i]
		}
	}
	t.Fatalf("%s has no exercise %s", req.Title, templateID)
	return -1, nil
}

func setWeights(sets []hevy.RoutineSetRequest) []float64 {
	weights := make([]float64, 0, len(sets))
	for _, s := range sets {
		weights = append(weights, *s.WeightKg)
	}
	return weights
}

func TestSyncerPlan_Cycle7(T *testing.T) {
	T.Parallel()

	T.Run("routines are titled by day in week order, with no bench", func(t *testing.T) {
		t.Parallel()

		var titles []string
		for _, p := range NewSyncer(nil, cycle7Config()).Plan() {
			titles = append(titles, p.Request.Title)
		}

		var expected []string
		for week := 1; week <= DeloadWeek; week++ {
			for _, day := range []string{"Squat", "Press", "Overhead Press", "Deadlift"} {
				expected = append(expected, fmt.Sprintf("C7W%d -- %s", week, day))
			}
		}
		assert.Equal(t, expected, titles)
	})

	T.Run("a disabled lift is not generated", func(t *testing.T) {
		t.Parallel()

		for _, p := range NewSyncer(nil, cycle7Config()).Plan() {
			assert.NotEqual(t, BenchPress, p.Day.Lift, p.Request.Title)
			for _, e := range p.Request.Exercises {
				assert.NotEqual(t, cycle7BenchID, e.ExerciseTemplateID, p.Request.Title)
			}
		}
	})

	T.Run("re-enabling a disabled lift puts it back in its slot", func(t *testing.T) {
		t.Parallel()

		cfg := cycle7Config()
		bench := cfg.Lifts[BenchPress]
		bench.Disabled = false
		cfg.Lifts[BenchPress] = bench

		plan := NewSyncer(nil, cfg).Plan()
		require.Len(t, plan, 5*DeloadWeek)
		assert.Equal(t, "C7W1 -- Bench Press", plan[1].Request.Title)
	})

	T.Run("the disabled lift keeps its training max and routine IDs", func(t *testing.T) {
		t.Parallel()

		cfg := cycle7Config()
		cfg.RoutineIDs = map[Lift]map[int]string{BenchPress: {1: "old-bench-routine"}}
		NewSyncer(nil, cfg).Plan()

		assert.InDelta(t, 70.0, cfg.Lifts[BenchPress].TrainingMaxKg, 0.001)
		assert.Equal(t, "old-bench-routine", cfg.RoutineIDs[BenchPress][1])
	})
}

func TestSyncerPlan_Cycle7MainSets(T *testing.T) {
	T.Parallel()

	// The main exercise's weights, set for set: the empty bar (not on the deadlift),
	// warmups, working sets, then FSL. These are the main-set weights the code produced
	// for these training maxes before the Cycle 7 changes, compared exactly.
	table := []struct {
		lift       Lift
		templateID string
		weights    []float64
		week       int
	}{
		{lift: Squat, templateID: cycle7SquatID, week: 1, weights: []float64{20, 37.5, 47.5, 57.5, 62.5, 72.5, 80, 57.5, 57.5, 57.5, 57.5, 57.5}},
		{lift: Squat, templateID: cycle7SquatID, week: 2, weights: []float64{20, 37.5, 47.5, 57.5, 67.5, 75, 85, 60, 60, 60, 60, 60}},
		{lift: Squat, templateID: cycle7SquatID, week: 3, weights: []float64{20, 37.5, 47.5, 57.5, 72.5, 80, 90, 65, 65, 65, 65, 65}},
		{lift: Squat, templateID: cycle7SquatID, week: 4, weights: []float64{20, 37.5, 47.5, 57.5}},
		{lift: OverheadPress, templateID: cycle7OHPID, week: 1, weights: []float64{20, 25, 30, 32.5, 37.5, 42.5, 32.5, 32.5, 32.5, 32.5, 32.5}},
		{lift: OverheadPress, templateID: cycle7OHPID, week: 2, weights: []float64{20, 25, 30, 35, 40, 45, 35, 35, 35, 35, 35}},
		{lift: OverheadPress, templateID: cycle7OHPID, week: 3, weights: []float64{20, 25, 30, 37.5, 42.5, 47.5, 37.5, 37.5, 37.5, 37.5, 37.5}},
		{lift: OverheadPress, templateID: cycle7OHPID, week: 4, weights: []float64{20, 20, 25, 30}},
		// Deadlift FSL stays at 3 sets.
		{lift: Deadlift, templateID: cycle7DeadliftID, week: 1, weights: []float64{55, 67.5, 80, 87.5, 102.5, 115, 87.5, 87.5, 87.5}},
		{lift: Deadlift, templateID: cycle7DeadliftID, week: 2, weights: []float64{55, 67.5, 80, 95, 107.5, 122.5, 95, 95, 95}},
		{lift: Deadlift, templateID: cycle7DeadliftID, week: 3, weights: []float64{55, 67.5, 80, 102.5, 115, 127.5, 102.5, 102.5, 102.5}},
		{lift: Deadlift, templateID: cycle7DeadliftID, week: 4, weights: []float64{55, 67.5, 80}},
	}

	byTitle := planByTitle(NewSyncer(nil, cycle7Config()).Plan())

	for i := range table {
		tc := &table[i]
		T.Run(fmt.Sprintf("%s week %d", tc.lift, tc.week), func(t *testing.T) {
			t.Parallel()

			req := byTitle[fmt.Sprintf("C7W%d -- %s", tc.week, tc.lift.DisplayName())]
			require.NotNil(t, req)

			_, main := exerciseByTemplate(t, req, tc.templateID)
			assert.Equal(t, tc.weights, setWeights(main.Sets))
		})
	}
}

func TestSyncerPlan_Cycle7Annotations(T *testing.T) {
	T.Parallel()

	const deadliftNotes = "AMRAP set: stop at RPE 8. Last 3 sets: stop FSL if any set exceeds RPE 8"

	table := []struct {
		title      string
		templateID string
		notes      string
	}{
		{title: "C7W1 -- Deadlift", templateID: cycle7DeadliftID, notes: deadliftNotes},
		{title: "C7W2 -- Deadlift", templateID: cycle7DeadliftID, notes: deadliftNotes},
		{title: "C7W3 -- Deadlift", templateID: cycle7DeadliftID, notes: deadliftNotes},
		// The deload has neither an AMRAP set nor FSL, so it carries neither note.
		{title: "C7W4 -- Deadlift", templateID: cycle7DeadliftID, notes: ""},
		// The squat's paused-FSL cue is unchanged.
		{title: "C7W1 -- Squat", templateID: cycle7SquatID, notes: "Last 5 sets: 2s pause at the bottom"},
		{title: "C7W4 -- Squat", templateID: cycle7SquatID, notes: ""},
		{title: "C7W1 -- Overhead Press", templateID: cycle7OHPID, notes: ""},
	}

	byTitle := planByTitle(NewSyncer(nil, cycle7Config()).Plan())

	for _, tc := range table {
		T.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			req := byTitle[tc.title]
			require.NotNil(t, req)

			_, main := exerciseByTemplate(t, req, tc.templateID)
			assert.Equal(t, tc.notes, main.Notes)
		})
	}
}

func TestSyncerPlan_Cycle7Accessories(T *testing.T) {
	T.Parallel()

	byTitle := planByTitle(NewSyncer(nil, cycle7Config()).Plan())

	T.Run("press day is 3 sets of 8-10 standing chest press with no prescribed weight", func(t *testing.T) {
		t.Parallel()

		for week := 1; week <= DeloadWeek; week++ {
			req := byTitle[fmt.Sprintf("C7W%d -- Press", week)]
			require.NotNil(t, req, "week %d", week)
			require.Len(t, req.Exercises, 1)

			press := req.Exercises[0]
			assert.Equal(t, cycle7PressID, press.ExerciseTemplateID)
			assert.Equal(t, "RPE 8, stop if pain", press.Notes)
			require.Len(t, press.Sets, 3)
			for _, s := range press.Sets {
				assert.Equal(t, &hevy.RepRange{Start: 8, End: 10}, s.RepRange)
				assert.Nil(t, s.WeightKg)
				assert.Nil(t, s.Reps)
			}
		}
	})

	T.Run("log clean follows the overhead press FSL on working weeks", func(t *testing.T) {
		t.Parallel()

		for week := 1; week < DeloadWeek; week++ {
			req := byTitle[fmt.Sprintf("C7W%d -- Overhead Press", week)]
			require.NotNil(t, req)

			mainIdx, _ := exerciseByTemplate(t, req, cycle7OHPID)
			cleanIdx, clean := exerciseByTemplate(t, req, cycle7LogCleanID)
			assert.Equal(t, mainIdx+1, cleanIdx, "week %d", week)
			assert.Equal(t, "technique practice, ~55-65 kg", clean.Notes)
			require.Len(t, clean.Sets, 4)
			for _, s := range clean.Sets {
				assert.Equal(t, &hevy.RepRange{Start: 1, End: 2}, s.RepRange)
				assert.Nil(t, s.WeightKg)
			}
		}
	})

	T.Run("log clean is left out of the deload", func(t *testing.T) {
		t.Parallel()

		req := byTitle["C7W4 -- Overhead Press"]
		require.NotNil(t, req)

		for _, e := range req.Exercises {
			assert.NotEqual(t, cycle7LogCleanID, e.ExerciseTemplateID)
		}
		// The other accessories still run on the deload.
		exerciseByTemplate(t, req, cycle7FacePullID)
	})
}

func TestAuxToExerciseRequest_WarmupSetsAndPounds(T *testing.T) {
	T.Parallel()

	kg := func(v float64) *float64 { return &v }
	reps := func(v int) *int { return &v }
	// Converted at run time, as the code does: a constant 240*kgPerLb is folded exactly at
	// compile time and can differ from the run-time product in the last bit.
	lbToKg := func(lb float64) *float64 { converted := lb * kgPerLb; return &converted }

	for name, tc := range map[string]struct {
		expected []hevy.RoutineSetRequest
		aux      AuxiliaryExercise
	}{
		"warmup sets lead as warmup-type sets": {
			aux: AuxiliaryExercise{
				WeightKg:   new(40.0),
				Sets:       2,
				Reps:       4,
				WarmupSets: []AuxiliarySet{{WeightKg: new(20.0), Reps: 5}, {WeightKg: new(30.0), Reps: 3}},
			},
			expected: []hevy.RoutineSetRequest{
				{Type: hevy.SetTypeWarmup, WeightKg: kg(20), Reps: reps(5)},
				{Type: hevy.SetTypeWarmup, WeightKg: kg(30), Reps: reps(3)},
				{Type: hevy.SetTypeNormal, WeightKg: kg(40), Reps: reps(4)},
				{Type: hevy.SetTypeNormal, WeightKg: kg(40), Reps: reps(4)},
			},
		},
		"a working rep range puts the warmups in range mode too": {
			aux: AuxiliaryExercise{
				Sets:        1,
				Reps:        8,
				RepRangeEnd: 10,
				WarmupSets:  []AuxiliarySet{{Reps: 5}},
			},
			expected: []hevy.RoutineSetRequest{
				{Type: hevy.SetTypeWarmup, RepRange: &hevy.RepRange{Start: 5, End: 5}},
				{Type: hevy.SetTypeNormal, RepRange: &hevy.RepRange{Start: 8, End: 10}},
			},
		},
		"pound weights are sent in kilograms": {
			aux: AuxiliaryExercise{SetScheme: []AuxiliarySet{{WeightLb: new(240.0), Reps: 8}, {WeightLb: new(195.0), Reps: 12}}},
			expected: []hevy.RoutineSetRequest{
				{Type: hevy.SetTypeNormal, WeightKg: lbToKg(240), Reps: reps(8)},
				{Type: hevy.SetTypeNormal, WeightKg: lbToKg(195), Reps: reps(12)},
			},
		},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, auxToExerciseRequest(tc.aux).Sets)
		})
	}
}

func TestSyncerPlan_CustomDayWarmup(T *testing.T) {
	T.Parallel()

	global := AuxiliaryExercise{ExerciseTemplateID: "global-warmup-id", Sets: 1, Reps: 10}
	own := AuxiliaryExercise{ExerciseTemplateID: "day-warmup-id", Sets: 1, Reps: 12}
	work := AuxiliaryExercise{ExerciseTemplateID: "work-id", Sets: 3, Reps: 5}

	for name, tc := range map[string]struct {
		globalWarmup []AuxiliaryExercise
		dayWarmup    []AuxiliaryExercise
		expected     []string
	}{
		"no warmup anywhere": {
			expected: []string{"work-id"},
		},
		"the program-wide warmup when the day has none": {
			globalWarmup: []AuxiliaryExercise{global},
			expected:     []string{"global-warmup-id", "work-id"},
		},
		"the day's warmup replaces the program-wide one": {
			globalWarmup: []AuxiliaryExercise{global},
			dayWarmup:    []AuxiliaryExercise{own},
			expected:     []string{"day-warmup-id", "work-id"},
		},
		"the day's warmup alone": {
			dayWarmup: []AuxiliaryExercise{own},
			expected:  []string{"day-warmup-id", "work-id"},
		},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{
				CycleNumber: 8,
				Warmup:      tc.globalWarmup,
				Days:        []Day{{Name: "Bench", Warmup: tc.dayWarmup, Exercises: []AuxiliaryExercise{work}}},
			}
			plan := NewSyncer(nil, cfg).Plan()
			require.Len(t, plan, DeloadWeek)

			for _, p := range plan {
				var ids []string
				for _, e := range p.Request.Exercises {
					ids = append(ids, e.ExerciseTemplateID)
				}
				assert.Equal(t, tc.expected, ids, p.Request.Title)
			}
		})
	}
}

func TestDropDuplicateWarmups(T *testing.T) {
	T.Parallel()

	warm := func(kg float64, reps int) CalculatedSet {
		return CalculatedSet{Type: hevy.SetTypeWarmup, WeightKg: kg, Reps: reps}
	}
	work := func(kg float64, reps int) CalculatedSet {
		return CalculatedSet{Type: hevy.SetTypeNormal, WeightKg: kg, Reps: reps}
	}

	for name, tc := range map[string]struct {
		sets     []CalculatedSet
		expected []CalculatedSet
	}{
		"no duplicates": {
			sets:     []CalculatedSet{warm(20, 5), warm(40, 5), work(65, 5)},
			expected: []CalculatedSet{warm(20, 5), warm(40, 5), work(65, 5)},
		},
		"empty bar and a 40% warmup that rounds to it keep only the empty bar": {
			sets:     []CalculatedSet{warm(20, 5), warm(20, 5), warm(27.5, 5), warm(32.5, 3), work(35, 5)},
			expected: []CalculatedSet{warm(20, 5), warm(27.5, 5), warm(32.5, 3), work(35, 5)},
		},
		"the first of the duplicates is kept, whatever its reps": {
			sets:     []CalculatedSet{warm(20, 5), warm(20, 3)},
			expected: []CalculatedSet{warm(20, 5)},
		},
		"non-adjacent duplicates": {
			sets:     []CalculatedSet{warm(20, 5), warm(30, 5), warm(20, 5)},
			expected: []CalculatedSet{warm(20, 5), warm(30, 5)},
		},
		"working sets at a warmup's weight are kept (the deload)": {
			sets:     []CalculatedSet{warm(20, 5), work(20, 5), work(27.5, 5), work(27.5, 5)},
			expected: []CalculatedSet{warm(20, 5), work(20, 5), work(27.5, 5), work(27.5, 5)},
		},
		"empty": {},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			input := slices.Clone(tc.sets)
			assert.Equal(t, tc.expected, dropDuplicateWarmups(input))
			assert.Equal(t, tc.sets, input, "the input is left as it was")
		})
	}
}

func TestAppendAuxRequests_Supersets(T *testing.T) {
	T.Parallel()

	ex := func(id, superset string) AuxiliaryExercise {
		return AuxiliaryExercise{ExerciseTemplateID: id, Superset: superset, Sets: 1, Reps: 10}
	}
	deloadless := func(id, superset string) AuxiliaryExercise {
		e := ex(id, superset)
		e.SkipDeload = true
		return e
	}

	for name, tc := range map[string]struct {
		existing []hevy.RoutineExerciseRequest
		list     []AuxiliaryExercise
		// expected is the superset ID per appended exercise; -1 means none.
		expected []int
		week     int
	}{
		"no labels": {
			list:     []AuxiliaryExercise{ex("a", ""), ex("b", "")},
			expected: []int{-1, -1},
			week:     1,
		},
		"one pair": {
			list:     []AuxiliaryExercise{ex("a", ""), ex("b", "A"), ex("c", "A"), ex("d", "")},
			expected: []int{-1, 0, 0, -1},
			week:     1,
		},
		"two pairs are numbered in order": {
			list:     []AuxiliaryExercise{ex("a", "A"), ex("b", "A"), ex("c", "B"), ex("d", "B")},
			expected: []int{0, 0, 1, 1},
			week:     1,
		},
		"labels only need to differ from their neighbors": {
			list:     []AuxiliaryExercise{ex("a", "B"), ex("b", "B"), ex("c", "A"), ex("d", "A")},
			expected: []int{0, 0, 1, 1},
			week:     1,
		},
		"a group of three": {
			list:     []AuxiliaryExercise{ex("a", "A"), ex("b", "A"), ex("c", "A")},
			expected: []int{0, 0, 0},
			week:     1,
		},
		"numbering continues after supersets already in the routine": {
			existing: []hevy.RoutineExerciseRequest{{ExerciseTemplateID: "x", SupersetID: new(0)}, {ExerciseTemplateID: "y", SupersetID: new(0)}},
			list:     []AuxiliaryExercise{ex("a", "A"), ex("b", "A")},
			expected: []int{1, 1},
			week:     1,
		},
		"a pair left with one exercise on the deload is a plain exercise": {
			list:     []AuxiliaryExercise{ex("a", "A"), deloadless("b", "A")},
			expected: []int{-1},
			week:     DeloadWeek,
		},
		"the same pair outside the deload": {
			list:     []AuxiliaryExercise{ex("a", "A"), deloadless("b", "A")},
			expected: []int{0, 0},
			week:     1,
		},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			got := appendAuxRequests(slices.Clone(tc.existing), tc.list, tc.week)
			appended := got[len(tc.existing):]
			require.Len(t, appended, len(tc.expected))
			for i, want := range tc.expected {
				if want < 0 {
					assert.Nil(t, appended[i].SupersetID, i)
				} else {
					require.NotNil(t, appended[i].SupersetID, i)
					assert.Equal(t, want, *appended[i].SupersetID, i)
				}
			}
		})
	}
}
