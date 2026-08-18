package fivethreeone

import (
	"context"
	"fmt"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
)

// emptyBarKg is the weight of a standard Olympic barbell, used for the empty-bar warmup
// set on lifts with EmptyBarWarmup enabled.
const emptyBarKg = 20.0

// Syncer updates Hevy routines to reflect the current 5/3/1 program state.
type Syncer struct {
	client *hevy.Client
	config *Config
}

// NewSyncer creates a new Syncer.
func NewSyncer(client *hevy.Client, config *Config) *Syncer {
	return &Syncer{client: client, config: config}
}

// SyncRoutines creates or updates all 16 Hevy routines (4 lifts × 4 weeks).
func (s *Syncer) SyncRoutines(ctx context.Context) error {
	if s.config.RoutineIDs == nil {
		s.config.RoutineIDs = make(map[Lift]map[int]string)
	}
	for week := 1; week <= 4; week++ {
		for _, lift := range AllLifts() {
			liftCfg, ok := s.config.Lifts[lift]
			if !ok {
				continue
			}
			if s.config.RoutineIDs[lift] == nil {
				s.config.RoutineIDs[lift] = make(map[int]string)
			}
			req := s.buildRoutineRequest(lift, liftCfg, week)
			if routineID, exists := s.config.RoutineIDs[lift][week]; exists {
				updateReq := *req
				updateReq.FolderID = nil
				_, err := s.client.UpdateRoutine(ctx, routineID, &updateReq)
				if err == nil {
					fmt.Printf("Updated routine for %s — %s\n", lift.DisplayName(), WeekName(week))
					continue
				}
				if !hevy.IsNotFound(err) {
					return fmt.Errorf("updating routine for %s week %d: %w", lift.DisplayName(), week, err)
				}
				fmt.Printf("Routine %s for %s — %s missing in Hevy; recreating\n", routineID, lift.DisplayName(), WeekName(week))
				delete(s.config.RoutineIDs[lift], week)
			}
			routine, err := s.client.CreateRoutine(ctx, req)
			if err != nil {
				return fmt.Errorf("creating routine for %s week %d: %w", lift.DisplayName(), week, err)
			}
			s.config.RoutineIDs[lift][week] = routine.ID
			fmt.Printf("Created routine for %s — %s (ID: %s)\n", lift.DisplayName(), WeekName(week), routine.ID)
		}
	}
	return nil
}

func (s *Syncer) buildRoutineRequest(lift Lift, liftCfg LiftConfig, week int) *hevy.RoutineRequest {
	weekName := WeekName(week)
	title := fmt.Sprintf("C%dW%d -- %s", s.config.CycleNumber, week, lift.DisplayName())

	sets := CalculateRoutineSets(liftCfg.TrainingMaxKg, week, liftCfg.UseLbs)

	// Warm up with the empty bar before the computed warmup/working sets (major lifts
	// except the deadlift).
	if liftCfg.EmptyBarWarmup {
		emptyBar := CalculatedSet{Type: hevy.SetTypeWarmup, WeightKg: emptyBarKg, Reps: 5}
		sets = append([]CalculatedSet{emptyBar}, sets...)
	}

	// Presets that live on the main exercise (the FSL family) simply extend its set list.
	assistance := s.config.AssistanceSchemeFor(lift)
	assistanceSets := CalculateAssistanceSets(lift, assistance, liftCfg.TrainingMaxKg, week, liftCfg.UseLbs)
	assistanceApplied := false
	if assistance.InMainExercise() && len(assistanceSets) > 0 {
		sets = append(sets, assistanceSets...)
		assistanceApplied = true
	}

	var routineSets []hevy.RoutineSetRequest
	for _, cs := range sets {
		weight := cs.WeightKg
		// Hevy treats rep_range as an exercise-level mode: if any set in an exercise
		// uses rep_range, the plain reps field is ignored on every other set. Since
		// the AMRAP set needs a range, encode all working sets as ranges (fixed-rep
		// sets collapse to start == end).
		// The AMRAP set is encoded as a wide rep range (its minimum up to 20); Hevy's
		// routine API rejects an rpe field on routine sets, so it isn't set here.
		repRange := &hevy.RepRange{Start: cs.Reps, End: cs.Reps}
		if cs.IsAMRAP {
			repRange.End = 20
		}
		routineSets = append(routineSets, hevy.RoutineSetRequest{
			Type:     cs.Type,
			WeightKg: &weight,
			RepRange: repRange,
		})
	}

	var exercises []hevy.RoutineExerciseRequest
	warmupExercises := liftCfg.Warmup
	if len(warmupExercises) == 0 {
		warmupExercises = s.config.Warmup
	}
	for _, w := range warmupExercises {
		exercises = append(exercises, auxToExerciseRequest(w))
	}

	// A preset carrying a form cue (paused FSL) puts it on the exercise its sets are
	// logged against — Hevy's routine sets have no notes field of their own.
	mainNotes := ""
	if assistanceApplied && assistance.InMainExercise() {
		if cue := assistance.Cue(); cue != "" {
			mainNotes = fmt.Sprintf("Last %d sets: %s", len(assistanceSets), cue)
		}
	}

	mainRestSeconds := 150
	exercises = append(exercises, hevy.RoutineExerciseRequest{
		ExerciseTemplateID: liftCfg.ExerciseTemplateID,
		RestSeconds:        &mainRestSeconds,
		Notes:              mainNotes,
		Sets:               routineSets,
	})

	// Presets that log against their own exercise (BBB) become an exercise of their own,
	// placed after the main lift and before the auxiliary work.
	if assistance == AssistanceBBB && len(assistanceSets) > 0 && liftCfg.BBBExerciseTemplateID != "" {
		assistanceRestSeconds := 60
		assistanceRoutineSets := make([]hevy.RoutineSetRequest, 0, len(assistanceSets))
		for i := range assistanceSets {
			w, reps := assistanceSets[i].WeightKg, assistanceSets[i].Reps
			assistanceRoutineSets = append(assistanceRoutineSets, hevy.RoutineSetRequest{
				Type:     assistanceSets[i].Type,
				WeightKg: &w,
				Reps:     &reps,
			})
		}
		exercises = append(exercises, hevy.RoutineExerciseRequest{
			ExerciseTemplateID: liftCfg.BBBExerciseTemplateID,
			RestSeconds:        &assistanceRestSeconds,
			Sets:               assistanceRoutineSets,
		})
		assistanceApplied = true
	}

	for _, aux := range liftCfg.AuxiliaryExercises {
		exercises = append(exercises, auxToExerciseRequest(aux))
	}

	for _, c := range liftCfg.Cooldown {
		exercises = append(exercises, auxToExerciseRequest(c))
	}

	notes := fmt.Sprintf("5/3/1 Cycle %d, %s", s.config.CycleNumber, weekName)
	if assistanceApplied {
		notes += fmt.Sprintf(" — %s", assistance.DisplayNameFor(lift))
	}

	return &hevy.RoutineRequest{
		Title:     title,
		FolderID:  s.config.FolderID,
		Notes:     notes,
		Exercises: exercises,
	}
}

const defaultAuxRestSeconds = 120

func auxToExerciseRequest(aux AuxiliaryExercise) hevy.RoutineExerciseRequest {
	auxSets := make([]hevy.RoutineSetRequest, 0, aux.Sets)
	for range aux.Sets {
		rs := hevy.RoutineSetRequest{Type: hevy.SetTypeNormal}
		if aux.DurationSeconds != nil {
			d := *aux.DurationSeconds
			rs.DurationSeconds = &d
		} else {
			reps := aux.Reps
			rs.Reps = &reps
		}
		if aux.WeightKg != nil {
			w := *aux.WeightKg
			rs.WeightKg = &w
		}
		auxSets = append(auxSets, rs)
	}
	restSeconds := defaultAuxRestSeconds
	if aux.RestSeconds != nil {
		restSeconds = *aux.RestSeconds
	}
	return hevy.RoutineExerciseRequest{
		ExerciseTemplateID: aux.ExerciseTemplateID,
		RestSeconds:        &restSeconds,
		Notes:              aux.Notes,
		Sets:               auxSets,
	}
}
