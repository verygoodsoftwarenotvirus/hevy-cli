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


