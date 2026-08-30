package strongimport

import (
	"testing"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepair(t *testing.T) {
	t.Parallel()

	t.Run("clears every duration and keeps everything else", func(t *testing.T) {
		t.Parallel()

		w := workout(defaultStampSecs,
			exerciseSpec{squatTemplate, "Squat (Barbell)", stamped(2)},
			exerciseSpec{plankTemplate, "Plank", stamped(3)},
		)
		w.Description = " "
		w.Exercises[0].Notes = "Weight is per-side"
		w.Exercises[0].Sets[0].WeightKg = new(88.45044)
		w.Exercises[0].Sets[0].Reps = new(25)
		w.Exercises[0].Sets[1].RPE = new(8.5)

		req, err := Repair(w)
		require.NoError(t, err)

		require.Len(t, req.Exercises, 2)
		assert.Equal(t, "Legs/Abs 1 (WFH)", req.Title)
		assert.Equal(t, " ", *req.Description)
		assert.Equal(t, "2024-10-16T11:59:03Z", req.StartTime)
		assert.Equal(t, "2024-10-16T15:34:03Z", req.EndTime)
		assert.Equal(t, "Weight is per-side", req.Exercises[0].Notes)
		assert.Equal(t, 88.45044, *req.Exercises[0].Sets[0].WeightKg)
		assert.Equal(t, 25, *req.Exercises[0].Sets[0].Reps)
		assert.InDelta(t, 8.5, *req.Exercises[0].Sets[1].RPE, 0.0001)

		for _, e := range req.Exercises {
			for _, s := range e.Sets {
				assert.Nil(t, s.DurationSeconds)
				assert.Equal(t, hevy.SetTypeNormal, s.Type)
			}
		}
	})

	t.Run("an integral distance survives the narrower request type", func(t *testing.T) {
		t.Parallel()

		w := workout(defaultStampSecs, exerciseSpec{ellipticalTmpl, "Elliptical Trainer", stamped(1)})
		w.Exercises[0].Sets[0].DistanceMeters = new(5000.0)

		req, err := Repair(w)
		require.NoError(t, err)
		assert.Equal(t, 5000, *req.Exercises[0].Sets[0].DistanceMeters)
	})

	// A write that silently truncated a distance would lose data the repair was never asked to
	// touch, so Repair refuses rather than rounding.
	t.Run("a fractional distance refuses rather than truncating", func(t *testing.T) {
		t.Parallel()

		w := workout(defaultStampSecs, exerciseSpec{ellipticalTmpl, "Elliptical Trainer", stamped(1)})
		w.Exercises[0].Sets[0].DistanceMeters = new(5000.5)

		_, err := Repair(w)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be sent as an integer")
	})
}
