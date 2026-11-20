package fivethreeone

import (
	"context"
	"fmt"
	"slices"
	"strings"

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

// PlannedRoutine is one routine the program calls for: a training day in a given week,
// with the request that creates or updates it in Hevy.
type PlannedRoutine struct {
	Request *hevy.RoutineRequest
	Day     Day
	Week    int
}

// Plan builds every routine the config calls for — each training day for each of the
// four weeks, week by week — without touching Hevy. SyncRoutines sends exactly these.
func (s *Syncer) Plan() []PlannedRoutine {
	days := s.config.TrainingDays()
	plan := make([]PlannedRoutine, 0, len(days)*DeloadWeek)
	for week := 1; week <= DeloadWeek; week++ {
		for _, day := range days {
			var req *hevy.RoutineRequest
			if day.IsCustom() {
				req = s.buildCustomRoutineRequest(&day, week)
			} else {
				req = s.buildRoutineRequest(day.Lift, s.config.Lifts[day.Lift], week)
			}
			plan = append(plan, PlannedRoutine{Day: day, Week: week, Request: req})
		}
	}
	return plan
}

// routineIDs returns the week → routine ID map a day's routines are tracked in, creating
// it if needed.
func (s *Syncer) routineIDs(day *Day) map[int]string {
	if day.IsCustom() {
		if s.config.CustomRoutineIDs == nil {
			s.config.CustomRoutineIDs = make(map[string]map[int]string)
		}
		if s.config.CustomRoutineIDs[day.Name] == nil {
			s.config.CustomRoutineIDs[day.Name] = make(map[int]string)
		}
		return s.config.CustomRoutineIDs[day.Name]
	}
	if s.config.RoutineIDs == nil {
		s.config.RoutineIDs = make(map[Lift]map[int]string)
	}
	if s.config.RoutineIDs[day.Lift] == nil {
		s.config.RoutineIDs[day.Lift] = make(map[int]string)
	}
	return s.config.RoutineIDs[day.Lift]
}

// SyncRoutines creates or updates every planned routine (each training day × 4 weeks).
func (s *Syncer) SyncRoutines(ctx context.Context) error {
	for _, planned := range s.Plan() {
		name, week, req := planned.Day.DisplayName(), planned.Week, planned.Request
		ids := s.routineIDs(&planned.Day)
		if routineID, exists := ids[week]; exists {
			updateReq := *req
			updateReq.FolderID = nil
			_, err := s.client.UpdateRoutine(ctx, routineID, &updateReq)
			if err == nil {
				fmt.Printf("Updated routine for %s — %s\n", name, WeekName(week))
				continue
			}
			if !hevy.IsNotFound(err) {
				return fmt.Errorf("updating routine for %s week %d: %w", name, week, err)
			}
			fmt.Printf("Routine %s for %s — %s missing in Hevy; recreating\n", routineID, name, WeekName(week))
			delete(ids, week)
		}
		routine, err := s.client.CreateRoutine(ctx, req)
		if err != nil {
			return fmt.Errorf("creating routine for %s week %d: %w", name, week, err)
		}
		ids[week] = routine.ID
		fmt.Printf("Created routine for %s — %s (ID: %s)\n", name, WeekName(week), routine.ID)
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
		sets = dropDuplicateWarmups(append([]CalculatedSet{emptyBar}, sets...))
	}

	// Presets that live on the main exercise (the FSL family) simply extend its set list.
	// The FSL family's set count can taper week by week (LiftConfig.FSLSetsByWeek).
	assistance := s.config.AssistanceSchemeFor(lift)
	fslSets := liftCfg.FSLSetsFor(lift, week)
	assistanceSets := CalculateAssistanceSetsWithCount(assistance, liftCfg.TrainingMaxKg, week, fslSets, liftCfg.UseLbs)
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
	exercises = appendAuxRequests(exercises, warmupExercises, week)

	// Set-level annotations — the configured AMRAP note, and the preset's form cue (paused
	// FSL) plus the configured assistance note — go on the exercise those sets are logged
	// against, since Hevy's routine sets have no notes field of their own.
	var mainNoteParts []string
	if liftCfg.AMRAPNote != "" && slices.ContainsFunc(sets, func(cs CalculatedSet) bool { return cs.IsAMRAP }) {
		mainNoteParts = append(mainNoteParts, "AMRAP set: "+liftCfg.AMRAPNote)
	}
	if assistanceApplied && assistance.InMainExercise() {
		if cues := assistanceCues(assistance, liftCfg); cues != "" {
			mainNoteParts = append(mainNoteParts, fmt.Sprintf("Last %d sets: %s", len(assistanceSets), cues))
		}
	}
	mainNotes := strings.Join(mainNoteParts, ". ")

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
			Notes:              assistanceCues(assistance, liftCfg),
			Sets:               assistanceRoutineSets,
		})
		assistanceApplied = true
	}

	exercises = appendAuxRequests(exercises, liftCfg.AuxiliaryExercises, week)
	exercises = appendAuxRequests(exercises, liftCfg.Cooldown, week)

	notes := fmt.Sprintf("5/3/1 Cycle %d, %s", s.config.CycleNumber, weekName)
	if assistanceApplied {
		notes += fmt.Sprintf(" — %s", assistance.DisplayNameForSets(fslSets))
	}

	return &hevy.RoutineRequest{
		Title:     title,
		FolderID:  s.config.FolderID,
		Notes:     notes,
		Exercises: exercises,
	}
}

// assistanceCues joins the preset's form cue with the lift's configured assistance note,
// either of which may be empty.
func assistanceCues(assistance AssistanceScheme, liftCfg LiftConfig) string {
	var cues []string
	if cue := assistance.Cue(); cue != "" {
		cues = append(cues, cue)
	}
	if liftCfg.AssistanceNote != "" {
		cues = append(cues, liftCfg.AssistanceNote)
	}
	return strings.Join(cues, "; ")
}

// buildCustomRoutineRequest builds a custom day's routine: the day's warmup (or the
// program-wide warmup when the day has none), then the day's own exercises. Nothing in it
// derives from a training max.
func (s *Syncer) buildCustomRoutineRequest(day *Day, week int) *hevy.RoutineRequest {
	warmupExercises := day.Warmup
	if len(warmupExercises) == 0 {
		warmupExercises = s.config.Warmup
	}
	exercises := appendAuxRequests(nil, warmupExercises, week)
	exercises = appendAuxRequests(exercises, day.Exercises, week)

	// The note leaves out "5/3/1": a custom day is fixed-weight, not driven by a training max.
	return &hevy.RoutineRequest{
		Title:     fmt.Sprintf("C%dW%d -- %s", s.config.CycleNumber, week, day.Name),
		FolderID:  s.config.FolderID,
		Notes:     fmt.Sprintf("Cycle %d, %s", s.config.CycleNumber, WeekName(week)),
		Exercises: exercises,
	}
}

// dropDuplicateWarmups removes each warmup set whose weight repeats an earlier warmup
// set's, keeping the first, e.g. when a percentage warmup rounds down to the empty bar
// that already leads the list. Working sets are never removed.
func dropDuplicateWarmups(sets []CalculatedSet) []CalculatedSet {
	seen := make(map[float64]bool)
	kept := sets[:0:0]
	for i := range sets {
		if sets[i].Type == hevy.SetTypeWarmup {
			if seen[sets[i].WeightKg] {
				continue
			}
			seen[sets[i].WeightKg] = true
		}
		kept = append(kept, sets[i])
	}
	return kept
}

// appendAuxRequests appends the routine exercise for each entry of list that belongs in
// the given week. A run of adjacent entries sharing a Superset label becomes one Hevy
// superset, numbered after any superset already in exercises; a run left with a single
// exercise that week (the rest skip the deload) is sent as a plain exercise.
func appendAuxRequests(exercises []hevy.RoutineExerciseRequest, list []AuxiliaryExercise, week int) []hevy.RoutineExerciseRequest {
	nextSuperset := 0
	for i := range exercises {
		if id := exercises[i].SupersetID; id != nil && *id >= nextSuperset {
			nextSuperset = *id + 1
		}
	}

	for i := 0; i < len(list); {
		j := i + 1
		if label := list[i].Superset; label != "" {
			for j < len(list) && list[j].Superset == label {
				j++
			}
		}

		start := len(exercises)
		for k := i; k < j; k++ {
			if list[k].InWeek(week) {
				exercises = append(exercises, auxToExerciseRequest(list[k]))
			}
		}
		if len(exercises)-start > 1 {
			for k := start; k < len(exercises); k++ {
				exercises[k].SupersetID = new(nextSuperset)
			}
			nextSuperset++
		}
		i = j
	}
	return exercises
}

const defaultAuxRestSeconds = 120

func auxToExerciseRequest(aux AuxiliaryExercise) hevy.RoutineExerciseRequest {
	// Warmup sets lead, logged as warmup-type sets, then the working sets.
	type typedSet struct {
		setType hevy.SetType
		set     AuxiliarySet
	}
	warmups, working := aux.PrescribedWarmupSets(), aux.PrescribedSets()
	prescribed := make([]typedSet, 0, len(warmups)+len(working))
	for i := range warmups {
		prescribed = append(prescribed, typedSet{set: warmups[i], setType: hevy.SetTypeWarmup})
	}
	for i := range working {
		prescribed = append(prescribed, typedSet{set: working[i], setType: hevy.SetTypeNormal})
	}

	// Hevy treats rep_range as an exercise-level mode (see buildRoutineRequest), so if any
	// set prescribes a range, every rep-based set is sent as one.
	useRepRange := slices.ContainsFunc(prescribed, func(p typedSet) bool { return p.set.RepRangeEnd > 0 })
	auxSets := make([]hevy.RoutineSetRequest, 0, len(prescribed))
	for i := range prescribed {
		p := &prescribed[i].set
		rs := hevy.RoutineSetRequest{Type: prescribed[i].setType}
		switch {
		case p.DistanceMeters != nil:
			// A carried distance has no rep count; the weight is added below.
			d := *p.DistanceMeters
			rs.DistanceMeters = &d
		case p.DurationSeconds != nil:
			d := *p.DurationSeconds
			rs.DurationSeconds = &d
		case useRepRange:
			rs.RepRange = &hevy.RepRange{Start: p.Reps, End: max(p.Reps, p.RepRangeEnd)}
		default:
			reps := p.Reps
			rs.Reps = &reps
		}
		if p.WeightKg != nil {
			w := *p.WeightKg
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
