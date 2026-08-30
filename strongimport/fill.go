package strongimport

import (
	"fmt"
	"sort"
	"strings"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
)

// DefaultFillNote marks a reconstructed duration as reconstructed.
//
// A filled value is an estimate wearing the same clothes as a measurement, and nothing in Hevy's
// data model distinguishes the two. The note is the only place the difference can be recorded, so
// filling without one is not an option the API surface offers.
const DefaultFillNote = "Duration estimated; the real value was lost in the 2024-11-23 Strong import."

// Fill describes one exercise's reconstructed per-set duration.
type Fill struct {
	// Exercise is the Hevy exercise title to fill.
	Exercise string
	// Seconds is written to every blank set of that exercise.
	Seconds int
}

// Gap is a run of blank sets on an exercise Hevy does time.
type Gap struct {
	WorkoutID string
	Title     string
	Exercise  string
	// LocalDate is the workout's date, for display.
	LocalDate string
	// Blank counts the sets with no duration.
	Blank int
	// ExerciseIndex identifies the exercise within the workout.
	ExerciseIndex int
}

// FindGaps returns every blank set on a duration-typed exercise.
//
// A duration-typed exercise with no duration is the one case Hevy's UI cannot render honestly: it
// draws a blank as 0:00, which asserts a hold of zero seconds rather than an unknown one. Every gap
// is reported, including ones no Fill covers, so that a value nobody supplied is visible as an
// omission rather than passed over in silence.
func FindGaps(workouts []hevy.Workout, lookup ExerciseTypeLookup) []Gap {
	var gaps []Gap

	for i := range workouts {
		w := &workouts[i]

		for j := range w.Exercises {
			e := &w.Exercises[j]

			exerciseType, known := lookup(e.ExerciseTemplateID)
			if !known || !durationTypes[exerciseType] {
				continue
			}

			blank := 0
			for k := range e.Sets {
				if e.Sets[k].DurationSeconds == nil {
					blank++
				}
			}

			if blank == 0 {
				continue
			}

			gaps = append(gaps, Gap{
				WorkoutID:     w.ID,
				Title:         w.Title,
				LocalDate:     w.StartTime.Local().Format("2006-01-02"),
				Exercise:      e.Title,
				ExerciseIndex: e.Index,
				Blank:         blank,
			})
		}
	}

	sort.Slice(gaps, func(i, j int) bool { return gaps[i].LocalDate < gaps[j].LocalDate })

	return gaps
}

// ApplyFills builds the update that writes reconstructed durations into w's blank sets.
//
// Only blank sets are touched. A set that already carries a duration is left exactly as it is,
// whether it was measured or restored, so running this twice cannot drift a real value toward an
// estimated one.
func ApplyFills(w *hevy.Workout, fills map[string]int, note string) (*hevy.WorkoutRequest, int, error) {
	req, err := buildRequest(w)
	if err != nil {
		return nil, 0, err
	}

	filled := 0

	for i := range w.Exercises {
		e := &w.Exercises[i]

		seconds, wanted := fills[e.Title]
		if !wanted {
			continue
		}

		touched := false
		for j := range e.Sets {
			if e.Sets[j].DurationSeconds != nil {
				continue
			}
			value := seconds
			req.Exercises[i].Sets[j].DurationSeconds = &value
			filled++
			touched = true
		}

		if !touched {
			continue
		}

		if note != "" && !strings.Contains(req.Exercises[i].Notes, note) {
			req.Exercises[i].Notes = strings.TrimSpace(strings.TrimSpace(req.Exercises[i].Notes) + " " + note)
		}
	}

	if filled == 0 {
		return nil, 0, fmt.Errorf("workout %s: nothing to fill", w.ID)
	}

	return req, filled, nil
}
