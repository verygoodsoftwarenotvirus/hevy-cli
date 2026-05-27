package fivethreeone

import (
	"context"
	"fmt"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
)

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

	sets := CalculateRoutineSets(liftCfg.TrainingMax(), week, liftCfg.UseLbs)

	var routineSets []hevy.RoutineSetRequest
	for _, cs := range sets {
		weight := cs.WeightKg
		// Hevy treats rep_range as an exercise-level mode: if any set in an exercise
		// uses rep_range, the plain reps field is ignored on every other set. Since
		// the AMRAP set needs a range, encode all working sets as ranges (fixed-rep
		// sets collapse to start == end).
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

	mainRestSeconds := 150
	exercises = append(exercises, hevy.RoutineExerciseRequest{
		ExerciseTemplateID: liftCfg.ExerciseTemplateID,
		RestSeconds:        &mainRestSeconds,
		Sets:               routineSets,
	})

	// BBB assistance: 5×10 at 50% TM, skipped on deload week.
	if week != 4 && liftCfg.BBBExerciseTemplateID != "" {
		round := RoundWeight
		if liftCfg.UseLbs {
			round = RoundWeightLbs
		}
		bbbWeight := round(liftCfg.TrainingMax() * 0.50)
		bbbRestSeconds := 60
		var bbbSets []hevy.RoutineSetRequest
		for range 5 {
			w := bbbWeight
			reps := 10
			bbbSets = append(bbbSets, hevy.RoutineSetRequest{
				Type:     hevy.SetTypeNormal,
				WeightKg: &w,
				Reps:     &reps,
			})
		}
		exercises = append(exercises, hevy.RoutineExerciseRequest{
			ExerciseTemplateID: liftCfg.BBBExerciseTemplateID,
			RestSeconds:        &bbbRestSeconds,
			Sets:               bbbSets,
		})
	}

	for _, aux := range liftCfg.AuxiliaryExercises {
		exercises = append(exercises, auxToExerciseRequest(aux))
	}

	for _, c := range liftCfg.Cooldown {
		exercises = append(exercises, auxToExerciseRequest(c))
	}

	return &hevy.RoutineRequest{
		Title:     title,
		FolderID:  s.config.FolderID,
		Notes:     fmt.Sprintf("5/3/1 Cycle %d, %s", s.config.CycleNumber, weekName),
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
		Sets:               auxSets,
	}
}
