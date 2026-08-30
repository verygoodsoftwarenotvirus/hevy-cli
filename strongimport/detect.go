// Package strongimport detects and repairs damage left by the Strong-to-Hevy workout import.
//
// The import mapped the wrong source field into each set's duration: instead of the per-set time
// Strong records for timed exercises, it wrote the *workout's* total elapsed time onto every set of
// every exercise. A three-and-a-half hour leg session therefore claims a three-and-a-half hour
// plank, three-and-a-half hour squats, and three-and-a-half hour calf raises.
//
// Hevy only renders duration for exercises whose template type is duration-based, so the damage is
// visible on planks and cardio and silently wrong everywhere else. It still pollutes any query that
// aggregates set duration, which is why the repair covers every set rather than the visible ones.
package strongimport

import (
	"time"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
)

// durationTypes are the exercise template types for which Hevy stores and displays a per-set time.
// A set on any other type carrying a duration at all is evidence of the import bug, because neither
// the Hevy app nor the Strong export can produce one.
var durationTypes = map[hevy.ExerciseType]bool{
	"duration":          true,
	"distance_duration": true,
	"floors_duration":   true,
	"steps_duration":    true,
}

// ExerciseTypeLookup reports the template type for an exercise template ID. It returns false when
// the ID is unknown, which Detect treats as "cannot corroborate" rather than as a match.
type ExerciseTypeLookup func(templateID string) (hevy.ExerciseType, bool)

// Finding describes one workout carrying the import's stamped-duration damage.
type Finding struct {
	// Start is the workout's start time, the field a human recognizes it by.
	Start time.Time
	// WorkoutID identifies the workout to repair.
	WorkoutID string
	// Title is the workout title, for display.
	Title string
	// VisibleExercises lists the duration-typed exercises whose bogus time Hevy actually renders.
	// It is empty for workouts where the damage exists but never surfaces in the UI.
	VisibleExercises []string
	// StampedSeconds is the value written onto every set: the workout's own elapsed time.
	StampedSeconds int
	// Sets counts the sets that will have their duration cleared.
	Sets int
}

// Detect reports whether w carries the import's stamped-duration signature.
//
// The signature has two independent parts, and both must hold:
//
//   - Every set in the workout has a duration, and all of them equal the workout's own elapsed
//     time. Sets timed by a human vary; a constant equal to the wall clock of the session does not
//     happen by accident.
//
//   - At least one of those sets belongs to an exercise Hevy does not time at all. A barbell squat
//     has no duration field to fill in either app, so a squat with a duration can only have been
//     written by something that was not paying attention to the exercise type.
//
// The second part is what keeps a legitimate single-set cardio workout safe: a lone thirty-minute
// elliptical set does span its whole session, and satisfies the first part on its own.
func Detect(w *hevy.Workout, lookup ExerciseTypeLookup) (Finding, bool) {
	elapsed := int(w.EndTime.Sub(w.StartTime).Seconds())
	if elapsed <= 0 {
		return Finding{}, false
	}

	var (
		sets        int
		corroborate bool
		visible     []string
	)

	for i := range w.Exercises {
		e := &w.Exercises[i]

		for j := range e.Sets {
			s := &e.Sets[j]
			if s.DurationSeconds == nil || *s.DurationSeconds != elapsed {
				return Finding{}, false
			}
			sets++
		}

		if len(e.Sets) == 0 {
			continue
		}

		switch exerciseType, known := lookup(e.ExerciseTemplateID); {
		case !known:
			// Unknown template: neither corroborates nor disqualifies.
		case durationTypes[exerciseType]:
			visible = append(visible, e.Title)
		default:
			corroborate = true
		}
	}

	if sets == 0 || !corroborate {
		return Finding{}, false
	}

	return Finding{
		WorkoutID:        w.ID,
		Title:            w.Title,
		Start:            w.StartTime,
		StampedSeconds:   elapsed,
		Sets:             sets,
		VisibleExercises: visible,
	}, true
}
