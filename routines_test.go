package hevy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutineWriteResponseDecode(T *testing.T) {
	T.Parallel()

	for name, tc := range map[string]struct {
		body    string
		id      string
		wantErr bool
	}{
		"bare object":       {body: `{"id":"abc","title":"C7W1 -- Squat"}`, id: "abc"},
		"one-element array": {body: `[{"id":"abc","title":"C7W1 -- Squat"}]`, id: "abc"},
		"empty array":       {body: `[]`, wantErr: true},
		"null":              {body: `null`, wantErr: true},
		"missing":           {body: ``, wantErr: true},
	} {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			r := routineWriteResponse{Routine: []byte(tc.body)}
			routine, err := r.decode()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.id, routine.ID)
		})
	}
}
