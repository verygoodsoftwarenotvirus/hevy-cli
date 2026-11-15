package fivethreeone

import (
	"fmt"
	"slices"
	"testing"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	cycle8SquatID          = "squat-id"
	cycle8OHPID            = "ohp-id"
	cycle8DeadliftID       = "deadlift-id"
	cycle8BenchID          = "bench-id"
	cycle8LegPressID       = "iso-lateral-leg-press-id"
	cycle8BeltSquatRDLID   = "belt-squat-rdl-id"
	cycle8InclineDBID      = "incline-db-press-id"
	cycle8PushdownID       = "triceps-pushdown-id"
	cycle8CableRowID       = "seated-cable-row-v-grip-id"
	cycle8PullApartID      = "band-pull-apart-id"
	cycle8BandFacePullID   = "banded-face-pull-id"
	cycle8BandPushdownID   = "banded-triceps-pushdown-id"
	cycle8DislocateID      = "shoulder-dislocate-id"
	cycle8FacePullID       = "face-pull-id"
	cycle8LogCleanID       = "log-clean-id"
	cycle8CableCrunchID    = "cable-crunch-id"
	cycle8LatPulldownID    = "lat-pulldown-id"
	cycle8InclineCurlID    = "seated-incline-curl-id"
	cycle8HammerCurlID     = "hammer-curl-id"
	cycle8DBLateralID      = "lateral-raise-dumbbell-id"
	cycle8DragCurlID       = "drag-curl-id"
	cycle8ChestRowID       = "chest-supported-row-id"
	cycle8RGPulldownID     = "reverse-grip-lat-pulldown-id"
	cycle8ReverseHyperID   = "reverse-hyper-id"
	cycle8GluteBridgeID    = "glute-bridge-id"
	cycle8HipFlexorID      = "hip-flexor-lunge-id"
	cycle8GobletSquatID    = "goblet-squat-id"
	cycle8ToeTouchID       = "toe-touches-id"
	cycle8BarbellRDLID     = "romanian-deadlift-barbell-id"
	cycle8PushUpID         = "push-up-id"
	cycle8BenchNote        = "controlled descent, no bounce; stop if pain; bump 2.5 kg only after a fully pain-free week"
	cycle8ReverseHyperNote = "controlled, 1s squeeze at top; RPE 8"
	cycle8RPE8             = "RPE 8"
)

// cycle8TrainingMaxes are the Cycle 8 training maxes (bench runs as a fixed-weight day).
var cycle8TrainingMaxes = map[Lift]float64{
	Squat:         100.0,
	OverheadPress: 52.5,
	Deadlift:      140.0,
}

// cycle8Warmup builds a single-set warm-up exercise.
func cycle8Warmup(templateID string, reps int, notes string, weightKg *float64) AuxiliaryExercise {
	return AuxiliaryExercise{ExerciseTemplateID: templateID, Notes: notes, WeightKg: weightKg, Sets: 1, Reps: reps}
}

// cycle8BandWarmup is the banded upper-body warm-up shared by the bench and OHP days.
func cycle8BandWarmup() []AuxiliaryExercise {
	return []AuxiliaryExercise{
		cycle8Warmup(cycle8PullApartID, 12, "", nil),
		cycle8Warmup(cycle8BandFacePullID, 12, "", nil),
		cycle8Warmup(cycle8BandPushdownID, 12, "", nil),
	}
}

// cycle8Config mirrors the Cycle 8 program: paused FSL on the squat, a per-week FSL taper
// on every lift, a fixed-weight bench day in place of the press day, a single-set warm-up
// list per day (banded work on the upper-body days), accessories with an RPE note each
// and supersets on every day, pound-loaded accessories, and no Log Clean.
func cycle8Config() *Config {
	return &Config{
		CycleNumber: 8,
		Assistance:  AssistanceFSL,
		Lifts: map[Lift]LiftConfig{
			Squat: {
				ExerciseTemplateID: cycle8SquatID,
				TrainingMaxKg:      100,
				EmptyBarWarmup:     true,
				Assistance:         AssistanceFSLPaused,
				FSLSetsByWeek:      map[int]int{1: 5, 2: 4, 3: 3},
				Warmup: []AuxiliaryExercise{
					cycle8Warmup(cycle8GluteBridgeID, 12, "1s pause at the top of each rep.", nil),
					cycle8Warmup(cycle8HipFlexorID, 8, "8 per side.", nil),
					cycle8Warmup(cycle8GobletSquatID, 8, "2-3s pause at the bottom, knees out.", new(10.0)),
				},
				AuxiliaryExercises: []AuxiliaryExercise{
					{
						Name:               "Iso-Lateral Leg Press",
						ExerciseTemplateID: cycle8LegPressID,
						Notes:              "RPE 8; don't let hips roll off pad",
						Sets:               3,
						SetScheme: []AuxiliarySet{
							{WeightLb: new(240.0), Reps: 8},
							{WeightLb: new(215.0), Reps: 10},
							{WeightLb: new(195.0), Reps: 12},
						},
					},
					{ExerciseTemplateID: cycle8ReverseHyperID, Notes: cycle8ReverseHyperNote, WeightLb: new(110.0), Sets: 3, Reps: 10, RepRangeEnd: 12},
					{ExerciseTemplateID: cycle8LatPulldownID, Notes: cycle8RPE8, Superset: "A", Sets: 3, Reps: 10, RepRangeEnd: 12},
					{ExerciseTemplateID: cycle8InclineCurlID, Notes: cycle8RPE8, Superset: "A", WeightLb: new(30.0), Sets: 3, Reps: 10, RepRangeEnd: 12},
				},
			},
			BenchPress: {
				ExerciseTemplateID: cycle8BenchID,
				TrainingMaxKg:      70,
				EmptyBarWarmup:     true,
				Disabled:           true,
			},
			OverheadPress: {
				ExerciseTemplateID: cycle8OHPID,
				TrainingMaxKg:      52.5,
				EmptyBarWarmup:     true,
				FSLSetsByWeek:      map[int]int{1: 5, 2: 4, 3: 3},
				Warmup:             append(cycle8BandWarmup(), cycle8Warmup(cycle8DislocateID, 10, "", nil)),
				AuxiliaryExercises: []AuxiliaryExercise{
					{ExerciseTemplateID: cycle8FacePullID, Notes: cycle8RPE8, Superset: "A", WeightKg: new(31.75), Sets: 3, Reps: 10},
					{ExerciseTemplateID: cycle8DBLateralID, Notes: cycle8RPE8, Superset: "A", WeightLb: new(20.0), Sets: 3, Reps: 12, RepRangeEnd: 15},
					{ExerciseTemplateID: cycle8PushdownID, Notes: cycle8RPE8, Superset: "B", WeightKg: new(58.0), Sets: 3, Reps: 10},
					{ExerciseTemplateID: cycle8DragCurlID, Notes: cycle8RPE8, Superset: "B", WeightKg: new(31.75), Sets: 3, Reps: 10},
				},
			},
			Deadlift: {
				ExerciseTemplateID: cycle8DeadliftID,
				TrainingMaxKg:      140,
				AMRAPNote:          "stop at RPE 9",
				AssistanceNote:     "stop FSL if any set exceeds RPE 8",
				FSLSetsByWeek:      map[int]int{1: 3, 2: 3, 3: 2},
				Warmup: []AuxiliaryExercise{
					cycle8Warmup(cycle8ToeTouchID, 15, "", nil),
					cycle8Warmup(cycle8HipFlexorID, 8, "8 per side.", nil),
					cycle8Warmup(cycle8BarbellRDLID, 10, "", new(20.0)),
				},
				AuxiliaryExercises: []AuxiliaryExercise{
					{
						Name:               "Belt Squat Romanian Deadlift",
						ExerciseTemplateID: cycle8BeltSquatRDLID,
						Notes:              "RPE 7-8",
						WeightLb:           new(90.0),
						Sets:               3,
						Reps:               8,
						RepRangeEnd:        10,
					},
					{ExerciseTemplateID: cycle8ChestRowID, Notes: cycle8RPE8, Superset: "A", Sets: 3, Reps: 10},
					{
						ExerciseTemplateID: cycle8RGPulldownID,
						Notes:              cycle8RPE8,
						Superset:           "A",
						Sets:               4,
						SetScheme: []AuxiliarySet{
							{WeightKg: new(54.43), Reps: 15},
							{WeightKg: new(63.5), Reps: 12},
							{WeightKg: new(68.04), Reps: 10},
							{WeightKg: new(72.57), Reps: 8},
						},
					},
				},
			},
		},
		Days: []Day{
			{Lift: Squat},
			{Name: "Bench", Warmup: append(cycle8BandWarmup(), cycle8Warmup(cycle8PushUpID, 10, "slow on the way down.", nil)), Exercises: []AuxiliaryExercise{
				{
					Name:               "Bench Press (Barbell)",
					ExerciseTemplateID: cycle8BenchID,
					Notes:              cycle8BenchNote,
					WeightKg:           new(55.0),
					Sets:               3,
					Reps:               5,
					WarmupSets: []AuxiliarySet{
						{WeightKg: new(20.0), Reps: 5},
						{WeightKg: new(40.0), Reps: 3},
					},
				},
				{ExerciseTemplateID: cycle8InclineDBID, Notes: cycle8RPE8, Superset: "A", Sets: 3, Reps: 8, RepRangeEnd: 10},
				{ExerciseTemplateID: cycle8CableRowID, Notes: cycle8RPE8, Superset: "A", Sets: 3, Reps: 10, RepRangeEnd: 12},
				{ExerciseTemplateID: cycle8PushdownID, Notes: cycle8RPE8, Superset: "B", Sets: 3, Reps: 10, RepRangeEnd: 15},
				{ExerciseTemplateID: cycle8HammerCurlID, Notes: cycle8RPE8, Superset: "B", Sets: 3, Reps: 10, RepRangeEnd: 12},
				{ExerciseTemplateID: cycle8CableCrunchID, Notes: cycle8RPE8, WeightLb: new(150.0), Sets: 3, Reps: 10},
			}},
			{Lift: OverheadPress},
			{Lift: Deadlift},
		},
	}
}

func TestCalculateRoutineSets_Cycle8TopSets(T *testing.T) {
	T.Parallel()

	table := []struct {
		lift Lift
		top  float64
		week int
	}{
		{lift: Squat, week: 1, top: 85},
		{lift: Squat, week: 2, top: 90},
		{lift: Squat, week: 3, top: 95},
		{lift: Deadlift, week: 1, top: 120},
		{lift: Deadlift, week: 2, top: 125},
		{lift: Deadlift, week: 3, top: 132.5},
		{lift: OverheadPress, week: 1, top: 45},
		{lift: OverheadPress, week: 2, top: 47.5},
		{lift: OverheadPress, week: 3, top: 50},
	}

	for _, tc := range table {
		T.Run(fmt.Sprintf("%s week %d", tc.lift, tc.week), func(t *testing.T) {
			t.Parallel()

			sets := CalculateRoutineSets(cycle8TrainingMaxes[tc.lift], tc.week, false)
			require.NotEmpty(t, sets)

			top := sets[len(sets)-1]
			assert.True(t, top.IsAMRAP)
			assert.Equal(t, tc.top, top.WeightKg)
		})
	}
}

func TestSyncerPlan_Cycle8(T *testing.T) {
	T.Parallel()

	T.Run("routines are titled by day in week order, with bench as day 2", func(t *testing.T) {
		t.Parallel()

		var titles []string
		for _, p := range NewSyncer(nil, cycle8Config()).Plan() {
			titles = append(titles, p.Request.Title)
		}

		var expected []string
		for week := 1; week <= DeloadWeek; week++ {
			for _, day := range []string{"Squat", "Bench", "Overhead Press", "Deadlift"} {
				expected = append(expected, fmt.Sprintf("C8W%d -- %s", week, day))
			}
		}
		assert.Equal(t, expected, titles)
	})

	T.Run("the disabled bench lift generates nothing", func(t *testing.T) {
		t.Parallel()

		// The barbell bench appears only on the fixed-weight bench day.
		for _, p := range NewSyncer(nil, cycle8Config()).Plan() {
			assert.NotEqual(t, BenchPress, p.Day.Lift, p.Request.Title)
			if !p.Day.IsCustom() {
				for _, e := range p.Request.Exercises {
					assert.NotEqual(t, cycle8BenchID, e.ExerciseTemplateID, p.Request.Title)
				}
			}
		}
	})

	T.Run("log clean is gone from every routine", func(t *testing.T) {
		t.Parallel()

		for _, p := range NewSyncer(nil, cycle8Config()).Plan() {
			for _, e := range p.Request.Exercises {
				assert.NotEqual(t, cycle8LogCleanID, e.ExerciseTemplateID, p.Request.Title)
			}
		}
	})
}

func TestSyncerPlan_Cycle8MainSets(T *testing.T) {
	T.Parallel()

	// The main exercise's weights, set for set: the empty bar (not on the deadlift),
	// warmups, working sets, then the tapering FSL sets.
	table := []struct {
		lift       Lift
		templateID string
		weights    []float64
		week       int
	}{
		{lift: Squat, templateID: cycle8SquatID, week: 1, weights: []float64{20, 40, 50, 60, 65, 75, 85, 57.5, 57.5, 57.5, 57.5, 57.5}},
		{lift: Squat, templateID: cycle8SquatID, week: 2, weights: []float64{20, 40, 50, 60, 70, 80, 90, 62.5, 62.5, 62.5, 62.5}},
		{lift: Squat, templateID: cycle8SquatID, week: 3, weights: []float64{20, 40, 50, 60, 75, 85, 95, 67.5, 67.5, 67.5}},
		{lift: Squat, templateID: cycle8SquatID, week: 4, weights: []float64{20, 40, 50, 60}},
		{lift: OverheadPress, templateID: cycle8OHPID, week: 1, weights: []float64{20, 27.5, 32.5, 35, 40, 45, 35, 35, 35, 35, 35}},
		{lift: OverheadPress, templateID: cycle8OHPID, week: 2, weights: []float64{20, 27.5, 32.5, 37.5, 42.5, 47.5, 37.5, 37.5, 37.5, 37.5}},
		{lift: OverheadPress, templateID: cycle8OHPID, week: 3, weights: []float64{20, 27.5, 32.5, 40, 45, 50, 40, 40, 40}},
		{lift: OverheadPress, templateID: cycle8OHPID, week: 4, weights: []float64{20, 20, 27.5, 32.5}},
		{lift: Deadlift, templateID: cycle8DeadliftID, week: 1, weights: []float64{55, 70, 85, 90, 105, 120, 90, 90, 90}},
		{lift: Deadlift, templateID: cycle8DeadliftID, week: 2, weights: []float64{55, 70, 85, 97.5, 112.5, 125, 97.5, 97.5, 97.5}},
		{lift: Deadlift, templateID: cycle8DeadliftID, week: 3, weights: []float64{55, 70, 85, 105, 120, 132.5, 105, 105}},
		{lift: Deadlift, templateID: cycle8DeadliftID, week: 4, weights: []float64{55, 70, 85}},
	}

	byTitle := planByTitle(NewSyncer(nil, cycle8Config()).Plan())

	for i := range table {
		tc := &table[i]
		T.Run(fmt.Sprintf("%s week %d", tc.lift, tc.week), func(t *testing.T) {
			t.Parallel()

			req := byTitle[fmt.Sprintf("C8W%d -- %s", tc.week, tc.lift.DisplayName())]
			require.NotNil(t, req)

			_, main := exerciseByTemplate(t, req, tc.templateID)
			assert.Equal(t, tc.weights, setWeights(main.Sets))
		})
	}
}

// TestSyncerPlan_Cycle8MainSetEncoding pins the shape every main-lift set is sent in —
// the same encoding Cycle 7 used — so the printout of the main 5/3/1 sets keeps its exact
// format: a rep range on every set (fixed reps as start == end), the AMRAP set open to 20,
// and no plain reps, duration, or RPE.
func TestSyncerPlan_Cycle8MainSetEncoding(T *testing.T) {
	T.Parallel()

	byTitle := planByTitle(NewSyncer(nil, cycle8Config()).Plan())
	mainIDs := map[Lift]string{Squat: cycle8SquatID, OverheadPress: cycle8OHPID, Deadlift: cycle8DeadliftID}

	for lift, templateID := range mainIDs {
		for week := 1; week <= DeloadWeek; week++ {
			T.Run(fmt.Sprintf("%s week %d", lift, week), func(t *testing.T) {
				t.Parallel()

				req := byTitle[fmt.Sprintf("C8W%d -- %s", week, lift.DisplayName())]
				require.NotNil(t, req)
				_, main := exerciseByTemplate(t, req, templateID)

				require.NotNil(t, main.RestSeconds)
				assert.Equal(t, 150, *main.RestSeconds)

				amraps := 0
				for _, s := range main.Sets {
					require.NotNil(t, s.RepRange)
					require.NotNil(t, s.WeightKg)
					assert.Nil(t, s.Reps)
					assert.Nil(t, s.DurationSeconds)
					assert.Nil(t, s.RPE)
					if s.RepRange.End != s.RepRange.Start {
						amraps++
						assert.Equal(t, 20, s.RepRange.End)
					}
				}
				if week == DeloadWeek {
					assert.Zero(t, amraps)
				} else {
					assert.Equal(t, 1, amraps)
				}
			})
		}
	}
}

func TestSyncerPlan_Cycle8FSLTaper(T *testing.T) {
	T.Parallel()

	const (
		squatNotes = "5/3/1 Cycle 8, Week %d — Paused First Set Last (%d×5 @ 90%% of FSL, 2s pause at the bottom)"
		fslNotes   = "5/3/1 Cycle 8, Week %d — First Set Last (%d×5 @ first working set)"
	)

	// sets is the number of trailing FSL sets on the main exercise; zero on the deload.
	table := []struct {
		lift          Lift
		templateID    string
		routineNotes  string
		exerciseNotes string
		weightKg      float64
		week          int
		sets          int
	}{
		{lift: Squat, templateID: cycle8SquatID, week: 1, sets: 5, weightKg: 57.5, routineNotes: fmt.Sprintf(squatNotes, 1, 5), exerciseNotes: "Last 5 sets: 2s pause at the bottom"},
		{lift: Squat, templateID: cycle8SquatID, week: 2, sets: 4, weightKg: 62.5, routineNotes: fmt.Sprintf(squatNotes, 2, 4), exerciseNotes: "Last 4 sets: 2s pause at the bottom"},
		{lift: Squat, templateID: cycle8SquatID, week: 3, sets: 3, weightKg: 67.5, routineNotes: fmt.Sprintf(squatNotes, 3, 3), exerciseNotes: "Last 3 sets: 2s pause at the bottom"},
		{lift: Squat, templateID: cycle8SquatID, week: 4, routineNotes: "5/3/1 Cycle 8, Deload"},
		{lift: OverheadPress, templateID: cycle8OHPID, week: 1, sets: 5, weightKg: 35, routineNotes: fmt.Sprintf(fslNotes, 1, 5)},
		{lift: OverheadPress, templateID: cycle8OHPID, week: 2, sets: 4, weightKg: 37.5, routineNotes: fmt.Sprintf(fslNotes, 2, 4)},
		{lift: OverheadPress, templateID: cycle8OHPID, week: 3, sets: 3, weightKg: 40, routineNotes: fmt.Sprintf(fslNotes, 3, 3)},
		{lift: OverheadPress, templateID: cycle8OHPID, week: 4, routineNotes: "5/3/1 Cycle 8, Deload"},
		{lift: Deadlift, templateID: cycle8DeadliftID, week: 1, sets: 3, weightKg: 90, routineNotes: fmt.Sprintf(fslNotes, 1, 3), exerciseNotes: "AMRAP set: stop at RPE 9. Last 3 sets: stop FSL if any set exceeds RPE 8"},
		{lift: Deadlift, templateID: cycle8DeadliftID, week: 2, sets: 3, weightKg: 97.5, routineNotes: fmt.Sprintf(fslNotes, 2, 3), exerciseNotes: "AMRAP set: stop at RPE 9. Last 3 sets: stop FSL if any set exceeds RPE 8"},
		{lift: Deadlift, templateID: cycle8DeadliftID, week: 3, sets: 2, weightKg: 105, routineNotes: fmt.Sprintf(fslNotes, 3, 2), exerciseNotes: "AMRAP set: stop at RPE 9. Last 2 sets: stop FSL if any set exceeds RPE 8"},
		{lift: Deadlift, templateID: cycle8DeadliftID, week: 4, routineNotes: "5/3/1 Cycle 8, Deload"},
	}

	byTitle := planByTitle(NewSyncer(nil, cycle8Config()).Plan())

	for i := range table {
		tc := &table[i]
		T.Run(fmt.Sprintf("%s week %d", tc.lift, tc.week), func(t *testing.T) {
			t.Parallel()

			liftCfg := cycle8Config().Lifts[tc.lift]
			if tc.week != DeloadWeek {
				assert.Equal(t, tc.sets, liftCfg.FSLSetsFor(tc.lift, tc.week))
			}

			req := byTitle[fmt.Sprintf("C8W%d -- %s", tc.week, tc.lift.DisplayName())]
			require.NotNil(t, req)
			assert.Equal(t, tc.routineNotes, req.Notes)

			_, main := exerciseByTemplate(t, req, tc.templateID)
			assert.Equal(t, tc.exerciseNotes, main.Notes)

			// The working sets end at the AMRAP set; everything after it is FSL.
			amrapIdx := -1
			for j, s := range main.Sets {
				if s.RepRange.End == 20 {
					amrapIdx = j
				}
			}
			var fsl []hevy.RoutineSetRequest
			if amrapIdx >= 0 {
				fsl = main.Sets[amrapIdx+1:]
			}
			require.Len(t, fsl, tc.sets)
			for _, s := range fsl {
				assert.Equal(t, hevy.SetTypeNormal, s.Type)
				assert.Equal(t, tc.weightKg, *s.WeightKg)
				assert.Equal(t, &hevy.RepRange{Start: 5, End: 5}, s.RepRange)
			}
		})
	}
}

func TestSyncerPlan_Cycle8LegPress(T *testing.T) {
	T.Parallel()

	byTitle := planByTitle(NewSyncer(nil, cycle8Config()).Plan())

	// The reverse pyramid as configured in pounds, and the kilograms sent to Hevy
	// (printed to one decimal).
	pyramid := []struct {
		printed string
		pounds  float64
		reps    int
	}{
		{pounds: 240, printed: "108.9", reps: 8},
		{pounds: 215, printed: "97.5", reps: 10},
		{pounds: 195, printed: "88.5", reps: 12},
	}

	for week := 1; week <= DeloadWeek; week++ {
		T.Run(fmt.Sprintf("week %d", week), func(t *testing.T) {
			t.Parallel()

			req := byTitle[fmt.Sprintf("C8W%d -- Squat", week)]
			require.NotNil(t, req)

			// It follows the squat (and its FSL) directly, ahead of the other accessories.
			mainIdx, _ := exerciseByTemplate(t, req, cycle8SquatID)
			pressIdx, press := exerciseByTemplate(t, req, cycle8LegPressID)
			hyperIdx, _ := exerciseByTemplate(t, req, cycle8ReverseHyperID)
			assert.Equal(t, mainIdx+1, pressIdx)
			assert.Equal(t, pressIdx+1, hyperIdx)

			assert.Equal(t, "RPE 8; don't let hips roll off pad", press.Notes)
			require.Len(t, press.Sets, len(pyramid))
			for j, rung := range pyramid {
				s := press.Sets[j]
				assert.Equal(t, hevy.SetTypeNormal, s.Type)
				require.NotNil(t, s.Reps)
				assert.Equal(t, rung.reps, *s.Reps)
				assert.Nil(t, s.RepRange)
				require.NotNil(t, s.WeightKg)
				// Unrounded, so it converts back to the exact pound figure in Hevy.
				assert.InDelta(t, rung.pounds, *s.WeightKg/kgPerLb, 1e-9)
				assert.Equal(t, rung.printed, fmt.Sprintf("%.1f", *s.WeightKg))
			}
		})
	}
}

func TestSyncerPlan_Cycle8BenchDay(T *testing.T) {
	T.Parallel()

	byTitle := planByTitle(NewSyncer(nil, cycle8Config()).Plan())

	// A fixed-weight day: the same touch-and-go bench every week, deload included, under a
	// routine note without "5/3/1" since nothing on it comes from a training max.
	for week := 1; week <= DeloadWeek; week++ {
		T.Run(fmt.Sprintf("week %d", week), func(t *testing.T) {
			t.Parallel()

			req := byTitle[fmt.Sprintf("C8W%d -- Bench", week)]
			require.NotNil(t, req)
			assert.Equal(t, fmt.Sprintf("Cycle 8, %s", WeekName(week)), req.Notes)

			_, bench := exerciseByTemplate(t, req, cycle8BenchID)
			assert.Equal(t, cycle8BenchNote, bench.Notes)
			assert.Nil(t, bench.SupersetID)
			assert.Equal(t, []hevy.RoutineSetRequest{
				cycle8Set(hevy.SetTypeWarmup, new(20.0), 5, 0),
				cycle8Set(hevy.SetTypeWarmup, new(40.0), 3, 0),
				cycle8Set(hevy.SetTypeNormal, new(55.0), 5, 0),
				cycle8Set(hevy.SetTypeNormal, new(55.0), 5, 0),
				cycle8Set(hevy.SetTypeNormal, new(55.0), 5, 0),
			}, bench.Sets)
		})
	}

	T.Run("the working weight is one config value", func(t *testing.T) {
		t.Parallel()

		cfg := cycle8Config()
		*cfg.Days[1].Exercises[0].WeightKg = 57.5

		req := planByTitle(NewSyncer(nil, cfg).Plan())["C8W1 -- Bench"]
		require.NotNil(t, req)
		_, bench := exerciseByTemplate(t, req, cycle8BenchID)
		assert.Equal(t, []float64{20, 40, 57.5, 57.5, 57.5}, setWeights(bench.Sets))
	})
}

// cycle8Set builds the set Hevy is sent: a fixed rep count, or a rep range when rangeEnd
// is set, at an optional weight.
func cycle8Set(setType hevy.SetType, weightKg *float64, reps, rangeEnd int) hevy.RoutineSetRequest {
	s := hevy.RoutineSetRequest{Type: setType, WeightKg: weightKg}
	if rangeEnd > 0 {
		s.RepRange = &hevy.RepRange{Start: reps, End: rangeEnd}
	} else {
		s.Reps = &reps
	}
	return s
}

// cycle8Sets is n identical working sets.
func cycle8Sets(n int, weightKg *float64, reps, rangeEnd int) []hevy.RoutineSetRequest {
	sets := make([]hevy.RoutineSetRequest, 0, n)
	for range n {
		sets = append(sets, cycle8Set(hevy.SetTypeNormal, weightKg, reps, rangeEnd))
	}
	return sets
}

func TestSyncerPlan_Cycle8Accessories(T *testing.T) {
	T.Parallel()

	byTitle := planByTitle(NewSyncer(nil, cycle8Config()).Plan())

	type accessory struct {
		superset   *int
		templateID string
		notes      string
		sets       []hevy.RoutineSetRequest
	}
	// Converted at run time, as the code does (see TestAuxToExerciseRequest_WarmupSetsAndPounds).
	pounds := func(lb float64) *float64 { kg := lb * kgPerLb; return &kg }
	legPress := []hevy.RoutineSetRequest{
		cycle8Set(hevy.SetTypeNormal, pounds(240), 8, 0),
		cycle8Set(hevy.SetTypeNormal, pounds(215), 10, 0),
		cycle8Set(hevy.SetTypeNormal, pounds(195), 12, 0),
	}
	reverseGripPyramid := []hevy.RoutineSetRequest{
		cycle8Set(hevy.SetTypeNormal, new(54.43), 15, 0),
		cycle8Set(hevy.SetTypeNormal, new(63.5), 12, 0),
		cycle8Set(hevy.SetTypeNormal, new(68.04), 10, 0),
		cycle8Set(hevy.SetTypeNormal, new(72.57), 8, 0),
	}

	// Everything after the day's main exercise (and its FSL), in order. Supersets are
	// numbered from 0 within each routine; every accessory carries an RPE note.
	table := []struct {
		day         string
		mainID      string
		accessories []accessory
	}{
		{
			day:    "Squat",
			mainID: cycle8SquatID,
			accessories: []accessory{
				{templateID: cycle8LegPressID, notes: "RPE 8; don't let hips roll off pad", sets: legPress},
				{templateID: cycle8ReverseHyperID, notes: cycle8ReverseHyperNote, sets: cycle8Sets(3, pounds(110), 10, 12)},
				{templateID: cycle8LatPulldownID, notes: cycle8RPE8, superset: new(0), sets: cycle8Sets(3, nil, 10, 12)},
				{templateID: cycle8InclineCurlID, notes: cycle8RPE8, superset: new(0), sets: cycle8Sets(3, pounds(30), 10, 12)},
			},
		},
		{
			day:    "Bench",
			mainID: cycle8BenchID,
			accessories: []accessory{
				{templateID: cycle8InclineDBID, notes: cycle8RPE8, superset: new(0), sets: cycle8Sets(3, nil, 8, 10)},
				{templateID: cycle8CableRowID, notes: cycle8RPE8, superset: new(0), sets: cycle8Sets(3, nil, 10, 12)},
				{templateID: cycle8PushdownID, notes: cycle8RPE8, superset: new(1), sets: cycle8Sets(3, nil, 10, 15)},
				{templateID: cycle8HammerCurlID, notes: cycle8RPE8, superset: new(1), sets: cycle8Sets(3, nil, 10, 12)},
				{templateID: cycle8CableCrunchID, notes: cycle8RPE8, sets: cycle8Sets(3, pounds(150), 10, 0)},
			},
		},
		{
			day:    "Overhead Press",
			mainID: cycle8OHPID,
			accessories: []accessory{
				{templateID: cycle8FacePullID, notes: cycle8RPE8, superset: new(0), sets: cycle8Sets(3, new(31.75), 10, 0)},
				{templateID: cycle8DBLateralID, notes: cycle8RPE8, superset: new(0), sets: cycle8Sets(3, pounds(20), 12, 15)},
				{templateID: cycle8PushdownID, notes: cycle8RPE8, superset: new(1), sets: cycle8Sets(3, new(58.0), 10, 0)},
				{templateID: cycle8DragCurlID, notes: cycle8RPE8, superset: new(1), sets: cycle8Sets(3, new(31.75), 10, 0)},
			},
		},
		{
			day:    "Deadlift",
			mainID: cycle8DeadliftID,
			accessories: []accessory{
				{templateID: cycle8BeltSquatRDLID, notes: "RPE 7-8", sets: cycle8Sets(3, pounds(90), 8, 10)},
				{templateID: cycle8ChestRowID, notes: cycle8RPE8, superset: new(0), sets: cycle8Sets(3, nil, 10, 0)},
				{templateID: cycle8RGPulldownID, notes: cycle8RPE8, superset: new(0), sets: reverseGripPyramid},
			},
		},
	}

	for _, tc := range table {
		for week := 1; week <= DeloadWeek; week++ {
			T.Run(fmt.Sprintf("%s week %d", tc.day, week), func(t *testing.T) {
				t.Parallel()

				req := byTitle[fmt.Sprintf("C8W%d -- %s", week, tc.day)]
				require.NotNil(t, req)

				mainIdx, _ := exerciseByTemplate(t, req, tc.mainID)
				got := req.Exercises[mainIdx+1:]
				require.Len(t, got, len(tc.accessories))
				for j, want := range tc.accessories {
					assert.Equal(t, want.templateID, got[j].ExerciseTemplateID, j)
					assert.Equal(t, want.notes, got[j].Notes, want.templateID)
					assert.Equal(t, want.superset, got[j].SupersetID, want.templateID)
					assert.Equal(t, want.sets, got[j].Sets, want.templateID)
				}
			})
		}
	}

	T.Run("nothing outside a superset pairing is grouped", func(t *testing.T) {
		t.Parallel()

		for _, p := range NewSyncer(nil, cycle8Config()).Plan() {
			counts := make(map[int]int)
			for _, e := range p.Request.Exercises {
				if e.SupersetID != nil {
					counts[*e.SupersetID]++
				}
			}
			for id, n := range counts {
				assert.Equal(t, 2, n, "%s superset %d", p.Request.Title, id)
			}
		}
	})
}

func TestSyncerPlan_Cycle8NoDuplicateWarmupWeights(T *testing.T) {
	T.Parallel()

	for _, p := range NewSyncer(nil, cycle8Config()).Plan() {
		T.Run(p.Request.Title, func(t *testing.T) {
			t.Parallel()

			for _, e := range p.Request.Exercises {
				seen := make(map[float64]bool)
				for _, s := range e.Sets {
					if s.Type != hevy.SetTypeWarmup || s.WeightKg == nil {
						continue
					}
					assert.False(t, seen[*s.WeightKg], "%s: two warmup sets at %.1f kg", e.ExerciseTemplateID, *s.WeightKg)
					seen[*s.WeightKg] = true
				}
			}
		})
	}
}

func TestSyncerPlan_Cycle8Warmups(T *testing.T) {
	T.Parallel()

	byTitle := planByTitle(NewSyncer(nil, cycle8Config()).Plan())

	type warmup struct {
		weightKg   *float64
		templateID string
		notes      string
		reps       int
	}
	band := []warmup{
		{templateID: cycle8PullApartID, reps: 12},
		{templateID: cycle8BandFacePullID, reps: 12},
		{templateID: cycle8BandPushdownID, reps: 12},
	}

	// Each day's warm-up exercises, in order, followed by the exercise they lead into.
	table := []struct {
		day    string
		mainID string
		warmup []warmup
	}{
		{
			day:    "Squat",
			mainID: cycle8SquatID,
			warmup: []warmup{
				{templateID: cycle8GluteBridgeID, notes: "1s pause at the top of each rep.", reps: 12},
				{templateID: cycle8HipFlexorID, notes: "8 per side.", reps: 8},
				{templateID: cycle8GobletSquatID, notes: "2-3s pause at the bottom, knees out.", reps: 8, weightKg: new(10.0)},
			},
		},
		{
			day:    "Bench",
			mainID: cycle8BenchID,
			warmup: append(slices.Clone(band), warmup{templateID: cycle8PushUpID, notes: "slow on the way down.", reps: 10}),
		},
		{
			day:    "Overhead Press",
			mainID: cycle8OHPID,
			warmup: append(slices.Clone(band), warmup{templateID: cycle8DislocateID, reps: 10}),
		},
		{
			day:    "Deadlift",
			mainID: cycle8DeadliftID,
			warmup: []warmup{
				{templateID: cycle8ToeTouchID, reps: 15},
				{templateID: cycle8HipFlexorID, notes: "8 per side.", reps: 8},
				{templateID: cycle8BarbellRDLID, reps: 10, weightKg: new(20.0)},
			},
		},
	}

	for _, tc := range table {
		for week := 1; week <= DeloadWeek; week++ {
			T.Run(fmt.Sprintf("%s week %d", tc.day, week), func(t *testing.T) {
				t.Parallel()

				req := byTitle[fmt.Sprintf("C8W%d -- %s", week, tc.day)]
				require.NotNil(t, req)

				// The warm-ups lead the routine, one set each, right before the main exercise.
				require.Greater(t, len(req.Exercises), len(tc.warmup))
				for j, want := range tc.warmup {
					got := req.Exercises[j]
					reps := want.reps
					assert.Equal(t, want.templateID, got.ExerciseTemplateID, j)
					assert.Equal(t, want.notes, got.Notes, want.templateID)
					assert.Equal(t, []hevy.RoutineSetRequest{
						{Type: hevy.SetTypeNormal, WeightKg: want.weightKg, Reps: &reps},
					}, got.Sets, want.templateID)
				}
				assert.Equal(t, tc.mainID, req.Exercises[len(tc.warmup)].ExerciseTemplateID)
			})
		}
	}

	T.Run("the overhead press barbell warm-ups start from the empty bar, once", func(t *testing.T) {
		t.Parallel()

		for week := 1; week < DeloadWeek; week++ {
			_, main := exerciseByTemplate(t, byTitle[fmt.Sprintf("C8W%d -- Overhead Press", week)], cycle8OHPID)
			assert.Equal(t, []float64{20, 27.5, 32.5}, setWeights(main.Sets)[:3], week)
			assert.Equal(t, hevy.SetTypeNormal, main.Sets[3].Type, week)
		}
	})
}
