package fivethreeone

import (
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
}
