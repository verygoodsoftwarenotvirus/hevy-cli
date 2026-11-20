package fivethreeone

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ptrTo returns a pointer to v, for filling the optional pointer fields on AuxiliaryExercise.
//
//go:fix inline
func ptrTo[T any](v T) *T {
	return new(v)
}

func TestAuxiliaryExercisePrescribedSets(T *testing.T) {
	T.Parallel()

	T.Run("expands the sets/reps/weight shorthand into identical sets", func(t *testing.T) {
		t.Parallel()

		aux := AuxiliaryExercise{WeightKg: new(61.0), Sets: 4, Reps: 10}

		prescribed := aux.PrescribedSets()
		require.Len(t, prescribed, 4)
		for _, p := range prescribed {
			require.NotNil(t, p.WeightKg)
			assert.InDelta(t, 61.0, *p.WeightKg, 0.001)
			assert.Equal(t, 10, p.Reps)
			assert.Nil(t, p.DurationSeconds)
		}
	})

	T.Run("carries the shorthand duration onto every set", func(t *testing.T) {
		t.Parallel()

		aux := AuxiliaryExercise{DurationSeconds: new(45), Sets: 2}

		prescribed := aux.PrescribedSets()
		require.Len(t, prescribed, 2)
		for _, p := range prescribed {
			require.NotNil(t, p.DurationSeconds)
			assert.Equal(t, 45, *p.DurationSeconds)
		}
	})

	T.Run("an explicit scheme prescribes each set on its own", func(t *testing.T) {
		t.Parallel()

		aux := AuxiliaryExercise{
			// A weight/reps shorthand is present too, to prove the scheme wins over it.
			WeightKg: new(61.0),
			Sets:     4,
			Reps:     10,
			SetScheme: []AuxiliarySet{
				{WeightKg: new(54.43), Reps: 15},
				{WeightKg: new(63.5), Reps: 12},
				{WeightKg: new(68.04), Reps: 10},
			},
		}

		prescribed := aux.PrescribedSets()
		require.Len(t, prescribed, 3)
		for i, expected := range []struct {
			weightKg float64
			reps     int
		}{{54.43, 15}, {63.5, 12}, {68.04, 10}} {
			require.NotNil(t, prescribed[i].WeightKg)
			assert.InDelta(t, expected.weightKg, *prescribed[i].WeightKg, 0.001)
			assert.Equal(t, expected.reps, prescribed[i].Reps)
		}
	})

	T.Run("scheme sets inherit the exercise's weight and reps when they omit them", func(t *testing.T) {
		t.Parallel()

		aux := AuxiliaryExercise{
			WeightKg: new(40.0),
			Reps:     8,
			SetScheme: []AuxiliarySet{
				{},
				{Reps: 6},
				{WeightKg: new(45.0)},
			},
		}

		prescribed := aux.PrescribedSets()
		require.Len(t, prescribed, 3)

		require.NotNil(t, prescribed[0].WeightKg)
		assert.InDelta(t, 40.0, *prescribed[0].WeightKg, 0.001)
		assert.Equal(t, 8, prescribed[0].Reps)

		require.NotNil(t, prescribed[1].WeightKg)
		assert.InDelta(t, 40.0, *prescribed[1].WeightKg, 0.001)
		assert.Equal(t, 6, prescribed[1].Reps)

		require.NotNil(t, prescribed[2].WeightKg)
		assert.InDelta(t, 45.0, *prescribed[2].WeightKg, 0.001)
		assert.Equal(t, 8, prescribed[2].Reps)
	})

	T.Run("filling a scheme set's weight leaves the configured scheme untouched", func(t *testing.T) {
		t.Parallel()

		aux := AuxiliaryExercise{WeightKg: new(40.0), Reps: 8, SetScheme: []AuxiliarySet{{}}}

		require.Len(t, aux.PrescribedSets(), 1)
		assert.Nil(t, aux.SetScheme[0].WeightKg)
		assert.Zero(t, aux.SetScheme[0].Reps)
	})

	T.Run("scheme sets inherit the exercise's rep range end", func(t *testing.T) {
		t.Parallel()

		aux := AuxiliaryExercise{Reps: 1, RepRangeEnd: 2, SetScheme: []AuxiliarySet{{}, {Reps: 3, RepRangeEnd: 5}}}

		prescribed := aux.PrescribedSets()
		require.Len(t, prescribed, 2)
		assert.Equal(t, AuxiliarySet{Reps: 1, RepRangeEnd: 2}, prescribed[0])
		assert.Equal(t, AuxiliarySet{Reps: 3, RepRangeEnd: 5}, prescribed[1])
	})

	T.Run("no sets at all prescribes nothing", func(t *testing.T) {
		t.Parallel()

		aux := AuxiliaryExercise{Reps: 10}

		assert.Empty(t, aux.PrescribedSets())
	})
}

func TestConfigTrainingDays(T *testing.T) {
	T.Parallel()

	T.Run("without days, one day per configured lift in standard order", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{Lifts: map[Lift]LiftConfig{Deadlift: {}, Squat: {}, BenchPress: {Disabled: true}}}
		assert.Equal(t, []Day{{Lift: Squat}, {Lift: Deadlift}}, cfg.TrainingDays())
	})

	T.Run("days set the order and disabled lifts drop out", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{
			Lifts: map[Lift]LiftConfig{Squat: {}, BenchPress: {Disabled: true}},
			Days:  []Day{{Lift: BenchPress}, {Name: "Press"}, {Lift: Squat}},
		}
		assert.Equal(t, []Day{{Name: "Press"}, {Lift: Squat}}, cfg.TrainingDays())
	})
}

func TestConfigValidate(T *testing.T) {
	T.Parallel()

	lifts := map[Lift]LiftConfig{Squat: {}}

	for name, tc := range map[string]struct {
		days  []Day
		valid bool
	}{
		"lift and custom days":     {days: []Day{{Lift: Squat}, {Name: "Press"}}, valid: true},
		"no days":                  {valid: true},
		"both lift and name":       {days: []Day{{Lift: Squat, Name: "Squat day"}}},
		"neither lift nor name":    {days: []Day{{}}},
		"lift missing from config": {days: []Day{{Lift: Deadlift}}},
		"unknown lift":             {days: []Day{{Lift: "benchpress"}}},
		"duplicate day":            {days: []Day{{Name: "Press"}, {Name: "Press"}}},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			err := (&Config{Lifts: lifts, Days: tc.days}).validate()
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAuxiliaryExerciseInWeek(T *testing.T) {
	T.Parallel()

	for name, tc := range map[string]struct {
		aux      AuxiliaryExercise
		week     int
		expected bool
	}{
		"working week":              {aux: AuxiliaryExercise{}, week: 1, expected: true},
		"deload":                    {aux: AuxiliaryExercise{}, week: DeloadWeek, expected: true},
		"skip deload, working week": {aux: AuxiliaryExercise{SkipDeload: true}, week: 3, expected: true},
		"skip deload, deload":       {aux: AuxiliaryExercise{SkipDeload: true}, week: DeloadWeek, expected: false},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, tc.aux.InWeek(tc.week))
		})
	}
}

func TestLiftConfigFSLSetsFor(T *testing.T) {
	T.Parallel()

	taper := map[int]int{1: 5, 2: 4, 3: 3}

	for name, tc := range map[string]struct {
		byWeek   map[int]int
		lift     Lift
		week     int
		expected int
	}{
		"configured week":                 {lift: Squat, byWeek: taper, week: 2, expected: 4},
		"configured final week":           {lift: OverheadPress, byWeek: taper, week: 3, expected: 3},
		"deadlift table overrides cap":    {lift: Deadlift, byWeek: map[int]int{1: 3, 2: 3, 3: 2}, week: 3, expected: 2},
		"no table uses the lift default":  {lift: Squat, week: 2, expected: 5},
		"no table, deadlift default cap":  {lift: Deadlift, week: 1, expected: 3},
		"week missing from partial table": {lift: Squat, byWeek: map[int]int{3: 2}, week: 1, expected: 5},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := LiftConfig{FSLSetsByWeek: tc.byWeek}
			assert.Equal(t, tc.expected, cfg.FSLSetsFor(tc.lift, tc.week))
		})
	}
}

func TestLiftConfigFSLSetsByWeekJSON(T *testing.T) {
	T.Parallel()

	var cfg LiftConfig
	require.NoError(T, json.Unmarshal([]byte(`{"fsl_sets_by_week": {"1": 3, "2": 3, "3": 2}}`), &cfg))
	assert.Equal(T, map[int]int{1: 3, 2: 3, 3: 2}, cfg.FSLSetsByWeek)
}

func TestAuxiliaryExercisePrescribedSets_Pounds(T *testing.T) {
	T.Parallel()

	kg := func(lb float64) *float64 { converted := lb * kgPerLb; return &converted }

	for name, tc := range map[string]struct {
		expected []AuxiliarySet
		aux      AuxiliaryExercise
	}{
		"exercise weight in pounds": {
			aux:      AuxiliaryExercise{WeightLb: new(70.0), Sets: 2, Reps: 10},
			expected: []AuxiliarySet{{WeightKg: kg(70), Reps: 10}, {WeightKg: kg(70), Reps: 10}},
		},
		"reverse pyramid in pounds": {
			aux: AuxiliaryExercise{SetScheme: []AuxiliarySet{
				{WeightLb: new(240.0), Reps: 8},
				{WeightLb: new(215.0), Reps: 10},
				{WeightLb: new(195.0), Reps: 12},
			}},
			expected: []AuxiliarySet{
				{WeightKg: kg(240), Reps: 8},
				{WeightKg: kg(215), Reps: 10},
				{WeightKg: kg(195), Reps: 12},
			},
		},
		"scheme set inherits a pound weight from the exercise": {
			aux:      AuxiliaryExercise{WeightLb: new(100.0), SetScheme: []AuxiliarySet{{Reps: 5}, {WeightKg: new(50.0), Reps: 3}}},
			expected: []AuxiliarySet{{WeightKg: kg(100), Reps: 5}, {WeightKg: new(50.0), Reps: 3}},
		},
		"scheme set in pounds overrides a kilogram exercise weight": {
			aux:      AuxiliaryExercise{WeightKg: new(50.0), SetScheme: []AuxiliarySet{{WeightLb: new(100.0), Reps: 5}}},
			expected: []AuxiliarySet{{WeightKg: kg(100), Reps: 5}},
		},
		"kilograms are passed through unchanged": {
			aux:      AuxiliaryExercise{WeightKg: new(61.0), Sets: 1, Reps: 10},
			expected: []AuxiliarySet{{WeightKg: new(61.0), Reps: 10}},
		},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, tc.aux.PrescribedSets())
		})
	}

	T.Run("converting leaves the configured scheme in pounds", func(t *testing.T) {
		t.Parallel()

		aux := AuxiliaryExercise{SetScheme: []AuxiliarySet{{WeightLb: new(240.0), Reps: 8}}}
		aux.PrescribedSets()
		assert.Nil(t, aux.SetScheme[0].WeightKg)
		assert.Equal(t, new(240.0), aux.SetScheme[0].WeightLb)
	})
}

func TestAuxiliaryExercisePrescribedWarmupSets(T *testing.T) {
	T.Parallel()

	lbToKg := func(lb float64) *float64 { converted := lb * kgPerLb; return &converted }

	for name, tc := range map[string]struct {
		expected []AuxiliarySet
		aux      AuxiliaryExercise
	}{
		"none configured": {
			aux:      AuxiliaryExercise{WeightKg: new(40.0), Sets: 3, Reps: 4},
			expected: []AuxiliarySet{},
		},
		"used as written, without the working weight or reps": {
			aux: AuxiliaryExercise{
				WeightKg:   new(40.0),
				Reps:       4,
				WarmupSets: []AuxiliarySet{{WeightKg: new(20.0), Reps: 5}, {WeightKg: new(30.0), Reps: 3}, {Reps: 5}},
			},
			expected: []AuxiliarySet{{WeightKg: new(20.0), Reps: 5}, {WeightKg: new(30.0), Reps: 3}, {Reps: 5}},
		},
		"pounds are converted": {
			aux:      AuxiliaryExercise{WarmupSets: []AuxiliarySet{{WeightLb: new(45.0), Reps: 5}}},
			expected: []AuxiliarySet{{WeightKg: lbToKg(45), Reps: 5}},
		},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, tc.aux.PrescribedWarmupSets())
		})
	}
}

func TestConfigValidate_LiftsAndExercises(T *testing.T) {
	T.Parallel()

	both := AuxiliaryExercise{Name: "Leg Press", WeightKg: new(100.0), WeightLb: new(220.0), Sets: 1, Reps: 8}
	bothInScheme := AuxiliaryExercise{Name: "Leg Press", SetScheme: []AuxiliarySet{{WeightKg: new(100.0), WeightLb: new(220.0)}}}
	bothInWarmup := AuxiliaryExercise{Name: "Bench", WarmupSets: []AuxiliarySet{{WeightKg: new(20.0), WeightLb: new(45.0)}}}
	pounds := AuxiliaryExercise{Name: "Leg Press", WeightLb: new(220.0), Sets: 1, Reps: 8}

	for name, tc := range map[string]struct {
		cfg   Config
		valid bool
	}{
		"fsl taper over weeks 1-3": {
			cfg:   Config{Lifts: map[Lift]LiftConfig{Squat: {FSLSetsByWeek: map[int]int{1: 5, 2: 4, 3: 3}}}},
			valid: true,
		},
		"fsl count on the deload week": {
			cfg: Config{Lifts: map[Lift]LiftConfig{Squat: {FSLSetsByWeek: map[int]int{DeloadWeek: 2}}}},
		},
		"fsl count on week 0": {
			cfg: Config{Lifts: map[Lift]LiftConfig{Squat: {FSLSetsByWeek: map[int]int{0: 2}}}},
		},
		"zero fsl sets": {
			cfg: Config{Lifts: map[Lift]LiftConfig{Deadlift: {FSLSetsByWeek: map[int]int{3: 0}}}},
		},
		"weight in pounds only": {
			cfg:   Config{Lifts: map[Lift]LiftConfig{Squat: {AuxiliaryExercises: []AuxiliaryExercise{pounds}}}},
			valid: true,
		},
		"accessory weight in both units": {
			cfg: Config{Lifts: map[Lift]LiftConfig{Squat: {AuxiliaryExercises: []AuxiliaryExercise{both}}}},
		},
		"scheme set weight in both units": {
			cfg: Config{Lifts: map[Lift]LiftConfig{Squat: {Cooldown: []AuxiliaryExercise{bothInScheme}}}},
		},
		"lift warmup weight in both units": {
			cfg: Config{Lifts: map[Lift]LiftConfig{OverheadPress: {Warmup: []AuxiliaryExercise{both}}}},
		},
		"global warmup weight in both units": {
			cfg: Config{Warmup: []AuxiliaryExercise{both}},
		},
		"custom day warmup set in both units": {
			cfg: Config{Days: []Day{{Name: "Bench", Exercises: []AuxiliaryExercise{bothInWarmup}}}},
		},
		"custom day warmup list": {
			cfg:   Config{Days: []Day{{Name: "Bench", Warmup: []AuxiliaryExercise{pounds}}}},
			valid: true,
		},
		"custom day warmup exercise weight in both units": {
			cfg: Config{Days: []Day{{Name: "Bench", Warmup: []AuxiliaryExercise{both}}}},
		},
		"a superset pair": {
			cfg: Config{Lifts: map[Lift]LiftConfig{Squat: {AuxiliaryExercises: []AuxiliaryExercise{
				{Name: "a", Superset: "A"}, {Name: "b", Superset: "A"}, {Name: "c"},
			}}}},
			valid: true,
		},
		"two supersets back to back": {
			cfg: Config{Days: []Day{{Name: "Bench", Exercises: []AuxiliaryExercise{
				{Name: "a", Superset: "A"}, {Name: "b", Superset: "A"}, {Name: "c", Superset: "B"}, {Name: "d", Superset: "B"},
			}}}},
			valid: true,
		},
		"the same label in different lists": {
			cfg: Config{Lifts: map[Lift]LiftConfig{Squat: {
				Warmup:             []AuxiliaryExercise{{Name: "a", Superset: "A"}, {Name: "b", Superset: "A"}},
				AuxiliaryExercises: []AuxiliaryExercise{{Name: "c", Superset: "A"}, {Name: "d", Superset: "A"}},
			}}},
			valid: true,
		},
		"a superset of one": {
			cfg: Config{Lifts: map[Lift]LiftConfig{Squat: {AuxiliaryExercises: []AuxiliaryExercise{
				{Name: "a", Superset: "A"}, {Name: "b"},
			}}}},
		},
		"a superset split by another exercise": {
			cfg: Config{Days: []Day{{Name: "Bench", Exercises: []AuxiliaryExercise{
				{Name: "a", Superset: "A"}, {Name: "b", Superset: "A"}, {Name: "c"}, {Name: "d", Superset: "A"}, {Name: "e", Superset: "A"},
			}}}},
		},
		"a superset in the global warmup with one exercise": {
			cfg: Config{Warmup: []AuxiliaryExercise{{Name: "a", Superset: "A"}}},
		},
		"warmup on a main-lift day": {
			cfg: Config{
				Lifts: map[Lift]LiftConfig{Squat: {}},
				Days:  []Day{{Lift: Squat, Warmup: []AuxiliaryExercise{pounds}}},
			},
		},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.cfg.validate()
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestDayWarmupJSON(T *testing.T) {
	T.Parallel()

	var day Day
	require.NoError(T, json.Unmarshal([]byte(`{"name":"Bench","warmup":[{"name":"Push Up","sets":1,"reps":10}]}`), &day))
	require.Len(T, day.Warmup, 1)
	assert.Equal(T, "Push Up", day.Warmup[0].Name)
	assert.Equal(T, 10, day.Warmup[0].Reps)
}

func TestAuxiliaryExerciseSupersetJSON(T *testing.T) {
	T.Parallel()

	var aux AuxiliaryExercise
	require.NoError(T, json.Unmarshal([]byte(`{"name":"Hammer Curl (Dumbbell)","superset":"B","sets":3,"reps":10}`), &aux))
	assert.Equal(T, "B", aux.Superset)

	data, err := json.Marshal(AuxiliaryExercise{Name: "Face Pull", Sets: 3})
	require.NoError(T, err)
	assert.NotContains(T, string(data), "superset", "an unlabeled exercise doesn't write the field")
}

func TestAuxiliaryExerciseValidate_DistanceAndPlaceholder(T *testing.T) {
	T.Parallel()

	for name, tc := range map[string]struct {
		aux   AuxiliaryExercise
		valid bool
	}{
		"a weighted distance":                {aux: AuxiliaryExercise{WeightLb: new(240.0), DistanceMeters: new(30), Sets: 3}, valid: true},
		"a zero distance":                    {aux: AuxiliaryExercise{DistanceMeters: new(0), Sets: 3}},
		"a negative distance in a scheme":    {aux: AuxiliaryExercise{SetScheme: []AuxiliarySet{{DistanceMeters: new(-5)}}}},
		"a weight placeholder":               {aux: AuxiliaryExercise{WeightPlaceholder: "LEG_CURL_KG", Sets: 3, Reps: 10}, valid: true},
		"a placeholder with weight_kg":       {aux: AuxiliaryExercise{WeightPlaceholder: "LEG_CURL_KG", WeightKg: new(40.0), Sets: 3, Reps: 10}},
		"a placeholder with weight_lb":       {aux: AuxiliaryExercise{WeightPlaceholder: "NEXT_PIN", WeightLb: new(160.0), Sets: 3, Reps: 10}},
		"a placeholder with a ramped scheme": {aux: AuxiliaryExercise{WeightPlaceholder: "TOP_SET", SetScheme: []AuxiliarySet{{WeightKg: new(20.0), Reps: 10}, {Reps: 8}}}, valid: true},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.aux.validate()
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAuxiliaryExerciseDistanceAndPlaceholderJSON(T *testing.T) {
	T.Parallel()

	var carry AuxiliaryExercise
	require.NoError(T, json.Unmarshal([]byte(`{"name":"Farmers Walk","weight_lb":240,"distance_meters":30,"sets":3}`), &carry))
	require.NotNil(T, carry.DistanceMeters)
	assert.Equal(T, 30, *carry.DistanceMeters)

	var curl AuxiliaryExercise
	require.NoError(T, json.Unmarshal([]byte(`{"name":"Seated Leg Curl (Machine)","weight_placeholder":"LEG_CURL_KG","sets":3,"reps":10}`), &curl))
	assert.Equal(T, "LEG_CURL_KG", curl.WeightPlaceholder)
	assert.Nil(T, curl.WeightKg)

	data, err := json.Marshal(AuxiliaryExercise{Name: "Face Pull", Sets: 3})
	require.NoError(T, err)
	assert.NotContains(T, string(data), "distance_meters")
	assert.NotContains(T, string(data), "weight_placeholder")
}

func TestConfigWeightPlaceholders(T *testing.T) {
	T.Parallel()

	open := AuxiliaryExercise{Name: "Seated Leg Curl (Machine)", WeightPlaceholder: "LEG_CURL_KG", Sets: 3, Reps: 10}
	filled := AuxiliaryExercise{Name: "Face Pull", WeightKg: new(30.0), Sets: 3, Reps: 10}

	for name, tc := range map[string]struct {
		expected []WeightPlaceholder
		cfg      Config
	}{
		"nothing open": {
			cfg: Config{Lifts: map[Lift]LiftConfig{Squat: {AuxiliaryExercises: []AuxiliaryExercise{filled}}}},
		},
		"on a lift's accessories": {
			cfg:      Config{Lifts: map[Lift]LiftConfig{Deadlift: {AuxiliaryExercises: []AuxiliaryExercise{filled, open}}}},
			expected: []WeightPlaceholder{{Day: "Deadlift", Exercise: "Seated Leg Curl (Machine)", Placeholder: "LEG_CURL_KG"}},
		},
		"on a custom day": {
			cfg:      Config{Days: []Day{{Name: "Bench", Exercises: []AuxiliaryExercise{open}}}},
			expected: []WeightPlaceholder{{Day: "Bench", Exercise: "Seated Leg Curl (Machine)", Placeholder: "LEG_CURL_KG"}},
		},
		"a disabled lift's are not reported": {
			cfg: Config{Lifts: map[Lift]LiftConfig{Deadlift: {Disabled: true, AuxiliaryExercises: []AuxiliaryExercise{open}}}},
		},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, tc.cfg.WeightPlaceholders())
		})
	}
}
