package fivethreeone

import (
	"fmt"
	"math"
	"strings"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
)

// PrescribedSet describes a single prescribed set in the 5/3/1 scheme.
type PrescribedSet struct {
	Type       hevy.SetType
	Percentage float64
	Reps       int
	IsAMRAP    bool
}

// WeekScheme describes the working sets for one week of 5/3/1.
type WeekScheme struct {
	Name string
	Sets []PrescribedSet
}

var warmupSets = []PrescribedSet{
	{Percentage: 0.40, Reps: 5, Type: hevy.SetTypeWarmup},
	{Percentage: 0.50, Reps: 5, Type: hevy.SetTypeWarmup},
	{Percentage: 0.60, Reps: 3, Type: hevy.SetTypeWarmup},
}

// DeloadWeek is the week within a cycle that drops volume and intensity; weeks 1-3 are
// working weeks.
const DeloadWeek = 4

var weekSchemes = map[int]WeekScheme{
	1: {Name: "Week 1", Sets: []PrescribedSet{
		{Percentage: 0.65, Reps: 5, Type: hevy.SetTypeNormal},
		{Percentage: 0.75, Reps: 5, Type: hevy.SetTypeNormal},
		{Percentage: 0.85, Reps: 5, IsAMRAP: true, Type: hevy.SetTypeNormal},
	}},
	2: {Name: "Week 2", Sets: []PrescribedSet{
		{Percentage: 0.70, Reps: 3, Type: hevy.SetTypeNormal},
		{Percentage: 0.80, Reps: 3, Type: hevy.SetTypeNormal},
		{Percentage: 0.90, Reps: 3, IsAMRAP: true, Type: hevy.SetTypeNormal},
	}},
	3: {Name: "Week 3", Sets: []PrescribedSet{
		{Percentage: 0.75, Reps: 5, Type: hevy.SetTypeNormal},
		{Percentage: 0.85, Reps: 3, Type: hevy.SetTypeNormal},
		{Percentage: 0.95, Reps: 1, IsAMRAP: true, Type: hevy.SetTypeNormal},
	}},
	4: {Name: "Deload", Sets: []PrescribedSet{
		{Percentage: 0.40, Reps: 5, Type: hevy.SetTypeNormal},
		{Percentage: 0.50, Reps: 5, Type: hevy.SetTypeNormal},
		{Percentage: 0.60, Reps: 5, Type: hevy.SetTypeNormal},
	}},
}

// AssistanceScheme names a preset for the supplemental volume that follows the main
// lift's working sets. Switching a program between presets is a one-word config change;
// each preset's sets are derived from the same training max as the working sets.
type AssistanceScheme string

const (
	// AssistanceBBB is Boring But Big: 5×10 at 50% of the training max, logged as a
	// separate exercise (see LiftConfig.BBBExerciseTemplateID).
	AssistanceBBB AssistanceScheme = "bbb"
	// AssistanceFSL is First Set Last: 5×5 at the week's first working-set weight,
	// appended to the main lift's own sets rather than split into its own exercise.
	// The deadlift runs fewer sets than the rest — see Lift.FSLSetCount.
	AssistanceFSL AssistanceScheme = "fsl"
	// AssistanceFSLPaused is First Set Last with a pause at the bottom of each rep,
	// run at a reduced fraction of the FSL weight (pausedFSLFactor) to keep the
	// slower, harder reps manageable. Intended as a temporary technique correction:
	// set it per-lift with LiftConfig.Assistance and drop the key to go back to
	// plain FSL.
	AssistanceFSLPaused AssistanceScheme = "fsl-paused"
	// AssistanceNone runs the main lift with no supplemental volume at all.
	AssistanceNone AssistanceScheme = "none"
)

// DefaultAssistanceScheme is used when a config doesn't name one. It is BBB so that
// configs written before presets existed keep their original behavior.
const DefaultAssistanceScheme = AssistanceBBB

// ParseAssistanceScheme resolves a user-supplied string to an assistance preset.
// Matching is case-insensitive and accepts each preset's spelled-out name.
func ParseAssistanceScheme(s string) (AssistanceScheme, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "bbb", "boring but big", "boring-but-big":
		return AssistanceBBB, true
	case "fsl", "first set last", "first-set-last":
		return AssistanceFSL, true
	case "fsl-paused", "fsl_paused", "paused", "paused fsl", "paused-fsl", "pfsl":
		return AssistanceFSLPaused, true
	case "none", "off", "":
		return AssistanceNone, true
	default:
		return "", false
	}
}

// DisplayName returns a human-readable name for the assistance preset.
func (a AssistanceScheme) DisplayName() string {
	switch a {
	case AssistanceBBB:
		return "Boring But Big (5×10 @ 50% TM)"
	case AssistanceFSL:
		return "First Set Last (5×5 @ first working set)"
	case AssistanceFSLPaused:
		return "Paused First Set Last (5×5 @ 90% of FSL)"
	case AssistanceNone:
		return "None"
	default:
		return string(a)
	}
}

// DisplayNameFor is DisplayName specialized to one lift, so presets whose shape varies
// by lift (the FSL family, where the deadlift runs fewer sets) describe what that lift
// actually does. Used where the name accompanies a single lift's sets; DisplayName
// remains the program-wide description.
func (a AssistanceScheme) DisplayNameFor(lift Lift) string {
	switch a {
	case AssistanceFSL:
		return fmt.Sprintf("First Set Last (%d×%d @ first working set)", lift.FSLSetCount(), fslReps)
	case AssistanceFSLPaused:
		return fmt.Sprintf("Paused First Set Last (%d×%d @ %d%% of FSL, %s)",
			lift.FSLSetCount(), fslReps, int(pausedFSLFactor*100), a.Cue())
	default:
		return a.DisplayName()
	}
}

// InMainExercise reports whether the preset's sets belong on the main lift's exercise
// rather than in a separate one.
func (a AssistanceScheme) InMainExercise() bool {
	return a == AssistanceFSL || a == AssistanceFSLPaused
}

// Cue returns the per-exercise form cue this preset's sets are performed with, or an
// empty string for presets that need no cue. Hevy's routine API carries notes on the
// exercise rather than the set (see hevy.RoutineSetRequest), so this is surfaced on the
// exercise the preset's sets are logged against.
func (a AssistanceScheme) Cue() string {
	if a == AssistanceFSLPaused {
		return fmt.Sprintf("%ds pause at the bottom", pausedFSLPauseSeconds)
	}
	return ""
}

const (
	bbbSetCount   = 5
	bbbReps       = 10
	bbbPercentage = 0.50

	fslSetCount         = 5
	fslDeadliftSetCount = 3
	fslReps             = 5

	// pausedFSLFactor scales the FSL weight for AssistanceFSLPaused. It is applied to
	// the rounded FSL weight — not the raw percentage — so the paused weight is always
	// visibly derived from the FSL weight printed alongside it.
	pausedFSLFactor = 0.90
	// pausedFSLPauseSeconds is how long the pause at the bottom of each rep is held.
	pausedFSLPauseSeconds = 2
)

// WeekName returns the display name for a given week number.
func WeekName(week int) string {
	if s, ok := weekSchemes[week]; ok {
		return s.Name
	}
	return fmt.Sprintf("Week %d", week)
}

// RoundWeight rounds a weight to the nearest 2.5 kg.
func RoundWeight(kg float64) float64 {
	return math.Round(kg/2.5) * 2.5
}

// RoundWeightLbs rounds a weight to the nearest 2.5 lbs, returned in kg.
func RoundWeightLbs(kg float64) float64 {
	const lbsPerKg = 2.20462
	lbs := kg * lbsPerKg
	rounded := math.Round(lbs/2.5) * 2.5
	return rounded / lbsPerKg
}

// CalculatedSet represents a fully computed set with weight and reps.
type CalculatedSet struct {
	Type     hevy.SetType
	WeightKg float64
	Reps     int
	IsAMRAP  bool
}

// rounder returns the weight-rounding function for the configured unit.
func rounder(useLbs bool) func(float64) float64 {
	if useLbs {
		return RoundWeightLbs
	}
	return RoundWeight
}

// CalculateRoutineSets computes the set list (warmups + working sets) for a given TM and week.
func CalculateRoutineSets(trainingMaxKg float64, week int, useLbs bool) []CalculatedSet {
	scheme, ok := weekSchemes[week]
	if !ok {
		return nil
	}

	round := rounder(useLbs)

	var sets []CalculatedSet

	// Warmup sets (skip during deload — deload already starts light)
	if week != DeloadWeek {
		for _, ws := range warmupSets {
			sets = append(sets, CalculatedSet{
				WeightKg: round(trainingMaxKg * ws.Percentage),
				Reps:     ws.Reps,
				Type:     ws.Type,
			})
		}
	}

	// Working sets
	for _, ps := range scheme.Sets {
		sets = append(sets, CalculatedSet{
			WeightKg: round(trainingMaxKg * ps.Percentage),
			Reps:     ps.Reps,
			IsAMRAP:  ps.IsAMRAP,
			Type:     ps.Type,
		})
	}

	return sets
}

// CalculateAssistanceSets computes the supplemental sets that follow the working sets for
// the given preset. Deload week gets none, whichever preset is configured — the point of
// the week is the reduced volume. The lift is needed because the FSL family varies its
// set count by lift (see Lift.FSLSetCount).
func CalculateAssistanceSets(lift Lift, scheme AssistanceScheme, trainingMaxKg float64, week int, useLbs bool) []CalculatedSet {
	weekScheme, ok := weekSchemes[week]
	if !ok || week == DeloadWeek {
		return nil
	}

	round := rounder(useLbs)

	var count, reps int
	var weight float64
	switch scheme {
	case AssistanceBBB:
		count, reps, weight = bbbSetCount, bbbReps, round(trainingMaxKg*bbbPercentage)
	case AssistanceFSL, AssistanceFSLPaused:
		if len(weekScheme.Sets) == 0 {
			return nil
		}
		// The FSL weight is the week's first working set, rounded exactly as that set
		// is. The paused variant then scales that already-rounded weight, so it stays a
		// visible fraction of the FSL number rather than drifting off the raw percentage.
		count, reps = lift.FSLSetCount(), fslReps
		weight = round(trainingMaxKg * weekScheme.Sets[0].Percentage)
		if scheme == AssistanceFSLPaused {
			weight = round(weight * pausedFSLFactor)
		}
	case AssistanceNone:
		return nil
	default:
		return nil
	}

	sets := make([]CalculatedSet, 0, count)
	for range count {
		sets = append(sets, CalculatedSet{
			WeightKg: weight,
			Reps:     reps,
			Type:     hevy.SetTypeNormal,
		})
	}
	return sets
}
