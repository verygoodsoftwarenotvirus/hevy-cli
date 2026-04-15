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
				if err != nil {
					return fmt.Errorf("updating routine for %s week %d: %w", lift.DisplayName(), week, err)
				}
				fmt.Printf("Updated routine for %s — %s\n", lift.DisplayName(), WeekName(week))
			} else {
				routine, err := s.client.CreateRoutine(ctx, req)
				if err != nil {
					return fmt.Errorf("creating routine for %s week %d: %w", lift.DisplayName(), week, err)
				}
				s.config.RoutineIDs[lift][week] = routine.ID
				fmt.Printf("Created routine for %s — %s (ID: %s)\n", lift.DisplayName(), WeekName(week), routine.ID)
			}
		}
	}
	return nil
}

// latestWorkoutsByRoutine fetches all workouts and returns a map of routine ID → most recent workout.
func (s *Syncer) latestWorkoutsByRoutine(ctx context.Context) (map[string]*hevy.Workout, error) {
	workouts, err := hevy.Collect(s.client.ListWorkouts(ctx))
	if err != nil {
		return nil, fmt.Errorf("listing workouts: %w", err)
	}
	latest := make(map[string]*hevy.Workout)
	for i := range workouts {
		w := &workouts[i]
		if w.RoutineID == nil {
			continue
		}
		rid := *w.RoutineID
		if prev, ok := latest[rid]; !ok || w.StartTime.After(prev.StartTime) {
			latest[rid] = w
		}
	}
	return latest, nil
}

// AdvanceCycle reads the latest week-3 workout for each lift, computes new training maxes
// via AMRAP performance, increments the cycle number, and updates the config in place.
func (s *Syncer) AdvanceCycle(ctx context.Context) error {
	byRoutine, err := s.latestWorkoutsByRoutine(ctx)
	if err != nil {
		return err
	}

	for _, lift := range AllLifts() {
		liftCfg, ok := s.config.Lifts[lift]
		if !ok {
			continue
		}

		week3ID, ok := s.config.RoutineIDs[lift][3]
		if !ok {
			fmt.Printf("%s: no week-3 routine ID configured, skipping\n", lift.DisplayName())
			continue
		}

		workout, ok := byRoutine[week3ID]
		if !ok {
			fmt.Printf("%s: no completed week-3 workout found, skipping\n", lift.DisplayName())
			continue
		}

		if len(workout.Exercises) == 0 || len(workout.Exercises[0].Sets) <= AMRAPSetIndex {
			fmt.Printf("%s: week-3 workout missing AMRAP set, skipping\n", lift.DisplayName())
			continue
		}

		amrapSet := workout.Exercises[0].Sets[AMRAPSetIndex]
		if amrapSet.WeightKg == nil || amrapSet.Reps == nil {
			fmt.Printf("%s: AMRAP set missing weight or reps, skipping\n", lift.DisplayName())
			continue
		}

		oldTM := liftCfg.TrainingMaxKg
		newTM := CalculateNewTM(oldTM, *amrapSet.WeightKg, *amrapSet.Reps)
		liftCfg.TrainingMaxKg = newTM
		s.config.Lifts[lift] = liftCfg
		fmt.Printf("%s: TM %.1f kg → %.1f kg (AMRAP: %.1f kg × %d reps)\n",
			lift.DisplayName(), oldTM, newTM, *amrapSet.WeightKg, *amrapSet.Reps)
	}

	s.config.CycleNumber++
	fmt.Printf("Advanced to cycle %d\n", s.config.CycleNumber)
	return nil
}

func (s *Syncer) buildRoutineRequest(lift Lift, liftCfg LiftConfig, week int) *hevy.RoutineRequest {
	weekName := WeekName(week)
	title := fmt.Sprintf("%s -- %s", weekName, lift.DisplayName())

	sets := CalculateRoutineSets(liftCfg.TrainingMaxKg, week, liftCfg.UseLbs)

	var routineSets []hevy.RoutineSetRequest
	for _, cs := range sets {
		weight := cs.WeightKg
		rs := hevy.RoutineSetRequest{
			Type:     cs.Type,
			WeightKg: &weight,
		}
		if cs.IsAMRAP {
			rs.RepRange = &hevy.RepRange{Start: cs.Reps, End: 20}
		} else {
			reps := cs.Reps
			rs.Reps = &reps
		}
		routineSets = append(routineSets, rs)
	}

	mainRestSeconds := 150
	exercises := []hevy.RoutineExerciseRequest{
		{
			ExerciseTemplateID: liftCfg.ExerciseTemplateID,
			RestSeconds:        &mainRestSeconds,
			Sets:               routineSets,
		},
	}

	// BBB assistance: 5×10 at 50% TM, skipped on deload week.
	if week != 4 && liftCfg.BBBExerciseTemplateID != "" {
		round := RoundWeight
		if liftCfg.UseLbs {
			round = RoundWeightLbs
		}
		bbbWeight := round(liftCfg.TrainingMaxKg * 0.50)
		bbbRestSeconds := 35
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

	return &hevy.RoutineRequest{
		Title:    title,
		FolderID: s.config.FolderID,
		Notes:    fmt.Sprintf("5/3/1 Cycle %d, %s", s.config.CycleNumber, weekName),
		Exercises: exercises,
	}
}
