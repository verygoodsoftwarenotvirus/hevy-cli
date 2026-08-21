package fivethreeone

import (
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

	T.Run("no sets at all prescribes nothing", func(t *testing.T) {
		t.Parallel()

		aux := AuxiliaryExercise{Reps: 10}

		assert.Empty(t, aux.PrescribedSets())
	})
}
