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
	AssistanceFSL AssistanceScheme = "fsl"
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
	case AssistanceNone:
		return "None"
	default:
		return string(a)
	}
}

// InMainExercise reports whether the preset's sets belong on the main lift's exercise
// rather than in a separate one.
func (a AssistanceScheme) InMainExercise() bool {
	return a == AssistanceFSL
}

const (
	bbbSetCount   = 5
	bbbReps       = 10
	bbbPercentage = 0.50

	fslSetCount = 5
	fslReps     = 5
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
// the week is the reduced volume.
func CalculateAssistanceSets(scheme AssistanceScheme, trainingMaxKg float64, week int, useLbs bool) []CalculatedSet {
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
	case AssistanceFSL:
		if len(weekScheme.Sets) == 0 {
			return nil
		}
		count, reps, weight = fslSetCount, fslReps, round(trainingMaxKg*weekScheme.Sets[0].Percentage)
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
