package fivethreeone

import (
	"fmt"
	"math"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
)

// PrescribedSet describes a single prescribed set in the 5/3/1 scheme.
type PrescribedSet struct {
	Percentage float64
	Reps       int
	IsAMRAP    bool
	Type       hevy.SetType
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
	WeightKg float64
	Reps     int
	IsAMRAP  bool
	Type     hevy.SetType
}

// CalculateRoutineSets computes the set list (warmups + working sets) for a given TM and week.
func CalculateRoutineSets(trainingMaxKg float64, week int, useLbs bool) []CalculatedSet {
	scheme, ok := weekSchemes[week]
	if !ok {
		return nil
	}

	round := RoundWeight
	if useLbs {
		round = RoundWeightLbs
	}

	var sets []CalculatedSet

	// Warmup sets (skip during deload — deload already starts light)
	if week != 4 {
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

// TMIncrement returns the training max increase after a cycle completes.
// Upper body: +2.5 kg, lower body: +5.0 kg.
func TMIncrement(lift Lift) float64 {
	if lift.IsUpperBody() {
		return 2.5
	}
	return 5.0
}

// AMRAPSetIndex returns the 0-based index of the AMRAP set within a workout exercise's
// set list. For weeks 1-3: 3 warmup sets + 3 working sets, AMRAP is the 6th (index 5).
const AMRAPSetIndex = 5

// CalculateNewOneRepMax estimates a new 1-rep max from an AMRAP performance
// using the Epley formula. The result is never less than the current 1RM.
//
// The conservative shave lives in TrainingMax() (TM = 90% × 1RM); this
// function returns the estimated 1RM itself.
func CalculateNewOneRepMax(currentOneRMKg, amrapWeightKg float64, amrapReps int) float64 {
	estimated1RM := RoundWeight(amrapWeightKg * (1 + float64(amrapReps)/30.0))
	if estimated1RM < currentOneRMKg {
		return currentOneRMKg
	}
	return estimated1RM
}
