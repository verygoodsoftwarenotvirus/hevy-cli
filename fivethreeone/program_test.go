package fivethreeone

import (
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

	T.Run("FSL matches the week's first working set", func(t *testing.T) {
		t.Parallel()

		// Week 1's first working set is 65% of the training max.
		sets := CalculateAssistanceSets(AssistanceFSL, trainingMax, 1, false)
		require.Len(t, sets, 5)
		for _, s := range sets {
			assert.InDelta(t, 65.0, s.WeightKg, 0.001)
			assert.Equal(t, 5, s.Reps)
			assert.Equal(t, hevy.SetTypeNormal, s.Type)
			assert.False(t, s.IsAMRAP)
		}

		// Week 3 opens at 75%, so its FSL sets are heavier.
		sets = CalculateAssistanceSets(AssistanceFSL, trainingMax, 3, false)
		require.Len(t, sets, 5)
		assert.InDelta(t, 75.0, sets[0].WeightKg, 0.001)
	})

	T.Run("BBB is 5x10 at half the training max", func(t *testing.T) {
		t.Parallel()

		sets := CalculateAssistanceSets(AssistanceBBB, trainingMax, 2, false)
		require.Len(t, sets, 5)
		for _, s := range sets {
			assert.InDelta(t, 50.0, s.WeightKg, 0.001)
			assert.Equal(t, 10, s.Reps)
		}
	})

	T.Run("no assistance on the deload week", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, CalculateAssistanceSets(AssistanceFSL, trainingMax, DeloadWeek, false))
		assert.Empty(t, CalculateAssistanceSets(AssistanceBBB, trainingMax, DeloadWeek, false))
	})

	T.Run("none and unknown weeks yield nothing", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, CalculateAssistanceSets(AssistanceNone, trainingMax, 1, false))
		assert.Empty(t, CalculateAssistanceSets(AssistanceFSL, trainingMax, 9, false))
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
