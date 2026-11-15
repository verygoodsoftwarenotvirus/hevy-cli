package hevy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutineExerciseSupersetDecode(T *testing.T) {
	T.Parallel()

	for name, tc := range map[string]struct {
		expected *int
		body     string
	}{
		"in a superset":     {body: `{"title":"Face Pull","superset_id":1}`, expected: new(1)},
		"the first group":   {body: `{"title":"Face Pull","superset_id":0}`, expected: new(0)},
		"not in a superset": {body: `{"title":"Face Pull","superset_id":null}`},
		"field absent":      {body: `{"title":"Face Pull"}`},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			var e RoutineExercise
			require.NoError(t, json.Unmarshal([]byte(tc.body), &e))
			assert.Equal(t, tc.expected, e.SupersetID)
		})
	}
}
