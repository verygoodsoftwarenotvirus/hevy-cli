package strongimport

import (
	"context"
	"fmt"
	"time"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
)

// Repair builds the update that clears the stamped durations from w.
//
// Every set's duration is dropped rather than corrected, because the stamped value carries no
// information about the set: it is the workout's elapsed time, which end_time minus start_time
// already gives. The genuine per-set times Strong recorded were overwritten at import and are not
// recoverable from Hevy, so a blank field is the honest result. Clearing them also restores the
// invariant Hevy itself maintains, that only duration-typed exercises carry a duration.
//
// Hevy's update endpoint replaces a workout wholesale, so anything the request model cannot express
// would be silently destroyed by a write. Rather than risk that, Repair refuses to build an update
// for a workout holding such a field and reports what it found.
func Repair(w *hevy.Workout) (*hevy.WorkoutRequest, error) {
	req, err := buildRequest(w)
	if err != nil {
		return nil, err
	}

	for i := range req.Exercises {
		for j := range req.Exercises[i].Sets {
			req.Exercises[i].Sets[j].DurationSeconds = nil
		}
	}

	return req, nil
}

// buildRequest re-expresses a workout as an update that changes nothing, so callers can edit one
// field without having to restate the other twenty.
func buildRequest(w *hevy.Workout) (*hevy.WorkoutRequest, error) {
	description := w.Description

	req := &hevy.WorkoutRequest{
		Title:       w.Title,
		Description: &description,
		StartTime:   w.StartTime.Format(time.RFC3339),
		EndTime:     w.EndTime.Format(time.RFC3339),
		IsPrivate:   w.IsPrivate,
		Exercises:   make([]hevy.WorkoutExerciseRequest, 0, len(w.Exercises)),
	}

	for i := range w.Exercises {
		e := &w.Exercises[i]

		exercise := hevy.WorkoutExerciseRequest{
			ExerciseTemplateID: e.ExerciseTemplateID,
			SupersetID:         e.SupersetID,
			Notes:              e.Notes,
			Sets:               make([]hevy.WorkoutSetRequest, 0, len(e.Sets)),
		}

		for j := range e.Sets {
			s := &e.Sets[j]

			set := hevy.WorkoutSetRequest{
				Type:            s.Type,
				WeightKg:        s.WeightKg,
				Reps:            s.Reps,
				RPE:             s.RPE,
				CustomMetric:    s.CustomMetric,
				DurationSeconds: s.DurationSeconds,
			}

			// The API returns distance as a float but only accepts an integer, so a fractional
			// distance cannot survive the round trip. None exist in the imported batch, but a write
			// that quietly truncated one would be worse than a write that did not happen.
			if s.DistanceMeters != nil {
				meters := int(*s.DistanceMeters)
				if float64(meters) != *s.DistanceMeters {
					return nil, fmt.Errorf(
						"workout %s exercise %d set %d: distance %g m cannot be sent as an integer",
						w.ID, e.Index, s.Index, *s.DistanceMeters,
					)
				}
				set.DistanceMeters = &meters
			}

			exercise.Sets = append(exercise.Sets, set)
		}

		req.Exercises = append(req.Exercises, exercise)
	}

	return req, nil
}

// Report summarizes a scan of the full workout history.
type Report struct {
	// Findings are the damaged workouts, in the order the API returned them.
	Findings []Finding
	// Scanned counts every workout read, damaged or not.
	Scanned int
}

// Sets counts the sets across every finding.
func (r *Report) Sets() int {
	var n int
	for i := range r.Findings {
		n += r.Findings[i].Sets
	}
	return n
}

// Visible counts the findings whose bogus duration Hevy actually displays.
func (r *Report) Visible() int {
	var n int
	for i := range r.Findings {
		if len(r.Findings[i].VisibleExercises) > 0 {
			n++
		}
	}
	return n
}

// Scan reads every workout and returns those carrying the import's signature.
func Scan(ctx context.Context, client *hevy.Client, lookup ExerciseTypeLookup) (*Report, error) {
	report := &Report{}

	for w, err := range client.ListWorkouts(ctx) {
		if err != nil {
			return nil, fmt.Errorf("listing workouts: %w", err)
		}

		report.Scanned++

		if finding, ok := Detect(&w, lookup); ok {
			report.Findings = append(report.Findings, finding)
		}
	}

	return report, nil
}

// LookupFromTemplates builds an ExerciseTypeLookup over every exercise template the account can
// see, custom templates included.
func LookupFromTemplates(ctx context.Context, client *hevy.Client) (ExerciseTypeLookup, error) {
	types := map[string]hevy.ExerciseType{}

	for t, err := range client.ListExerciseTemplates(ctx) {
		if err != nil {
			return nil, fmt.Errorf("listing exercise templates: %w", err)
		}
		types[t.ID] = t.Type
	}

	return func(id string) (hevy.ExerciseType, bool) {
		t, ok := types[id]
		return t, ok
	}, nil
}
