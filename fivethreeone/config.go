package fivethreeone

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
)

// Lift represents one of the four main 5/3/1 lifts.
type Lift string

const (
	Squat         Lift = "squat"
	BenchPress    Lift = "bench_press"
	OverheadPress Lift = "overhead_press"
	Deadlift      Lift = "deadlift"
)

// AllLifts returns all four main lifts in standard order.
func AllLifts() []Lift {
	return []Lift{Squat, BenchPress, OverheadPress, Deadlift}
}

// DisplayName returns a human-readable name for the lift.
func (l Lift) DisplayName() string {
	switch l {
	case Squat:
		return "Squat"
	case BenchPress:
		return "Bench Press"
	case OverheadPress:
		return "Overhead Press"
	case Deadlift:
		return "Deadlift"
	default:
		return string(l)
	}
}

// HevyBBBTitle returns the Hevy exercise title for this lift's BBB assistance work.
func (l Lift) HevyBBBTitle() string {
	switch l {
	case Squat:
		return "Squat (BBB Assistance)"
	case BenchPress:
		return "Bench Press (BBB Assistance)"
	case OverheadPress:
		return "Overhead Press (BBB Assistance)"
	case Deadlift:
		return "Deadlift (BBB Assistance)"
	default:
		return string(l)
	}
}

// HevyTitle returns the canonical Hevy exercise library title for this lift.
func (l Lift) HevyTitle() string {
	switch l {
	case Squat:
		return "High Bar Squat"
	case BenchPress:
		return "Bench Press (Barbell)"
	case OverheadPress:
		return "Overhead Press (Barbell)"
	case Deadlift:
		return "Deadlift (Barbell)"
	default:
		return string(l)
	}
}

// IsUpperBody returns true for upper body lifts.
func (l Lift) IsUpperBody() bool {
	return l == BenchPress || l == OverheadPress
}

// AuxiliaryExercise describes a user-supplied accessory movement appended to a lift's
// routine on every week (including deload). Either Reps or DurationSeconds should be set.
// Weight and rest are optional.
type AuxiliaryExercise struct {
	Name               string   `json:"name,omitempty"`
	ExerciseTemplateID string   `json:"exercise_template_id"`
	Sets               int      `json:"sets"`
	Reps               int      `json:"reps,omitempty"`
	DurationSeconds    *int     `json:"duration_seconds,omitempty"`
	WeightKg           *float64 `json:"weight_kg,omitempty"`
	RestSeconds        *int     `json:"rest_seconds,omitempty"`
}

// LiftConfig holds the 1-rep max and exercise template IDs for one lift.
type LiftConfig struct {
	OneRepMaxKg           float64             `json:"one_rep_max_kg"`
	ExerciseTemplateID    string              `json:"exercise_template_id"`
	BBBExerciseTemplateID string              `json:"bbb_exercise_template_id"`
	Warmup                []AuxiliaryExercise `json:"warmup,omitempty"`
	AuxiliaryExercises    []AuxiliaryExercise `json:"auxiliary_exercises,omitempty"`
	Cooldown              []AuxiliaryExercise `json:"cooldown,omitempty"`
	UseLbs                bool                `json:"use_lbs,omitempty"`
}

// TrainingMax returns the training max (TM) in kg: 90% of the stored 1-rep max.
//
// In Jim Wendler's 5/3/1, the TM is defined as 90% of the true 1RM, and every
// working-set, warmup, and BBB assistance percentage is applied to the TM —
// never to the 1RM directly. For example, a "65%" Week 1 set is
// 65% of TM = 65% × (90% × 1RM) = 58.5% of 1RM.
func (c LiftConfig) TrainingMax() float64 {
	return c.OneRepMaxKg * 0.9
}

// Config holds the complete 5/3/1 program state.
type Config struct {
	Lifts       map[Lift]LiftConfig     `json:"lifts"`
	CycleNumber int                     `json:"cycle_number"`
	WeekNumber  int                     `json:"week_number"`
	RoutineIDs  map[Lift]map[int]string `json:"routine_ids,omitempty"`
	FolderID    *int                    `json:"folder_id,omitempty"`
	Warmup      []AuxiliaryExercise     `json:"warmup,omitempty"`
}

// LoadConfig reads a Config from a JSON file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// findTemplateByTitle searches all exercise templates for one matching the given title
// (case-insensitive) and returns its ID.
func findTemplateByTitle(ctx context.Context, client *hevy.Client, title string) (string, error) {
	want := strings.ToLower(title)
	for tmpl, err := range client.ListExerciseTemplates(ctx) {
		if err != nil {
			return "", fmt.Errorf("listing exercise templates: %w", err)
		}
		if strings.ToLower(tmpl.Title) == want {
			return tmpl.ID, nil
		}
	}
	return "", fmt.Errorf("exercise template %q not found", title)
}

// FindExerciseTemplateID searches for the main lift exercise template.
func FindExerciseTemplateID(ctx context.Context, client *hevy.Client, lift Lift) (string, error) {
	return findTemplateByTitle(ctx, client, lift.HevyTitle())
}

// FindBBBExerciseTemplateID searches for the BBB assistance exercise template.
func FindBBBExerciseTemplateID(ctx context.Context, client *hevy.Client, lift Lift) (string, error) {
	return findTemplateByTitle(ctx, client, lift.HevyBBBTitle())
}

// RefreshExerciseTemplateIDs re-resolves exercise template IDs in the config from the
// Hevy API. It re-resolves BBB assistance IDs (which have known canonical titles) and
// any auxiliary/warmup/cooldown exercises that have a Name field set.
//
// Main lift IDs are intentionally left alone — they were chosen interactively at init
// time and may not match the hardcoded canonical titles. If a main lift ID has gone
// stale, re-run 'hevy 531 init' to reset the config.
func RefreshExerciseTemplateIDs(ctx context.Context, client *hevy.Client, cfg *Config) error {
	for _, lift := range AllLifts() {
		liftCfg, ok := cfg.Lifts[lift]
		if !ok {
			continue
		}

		if liftCfg.BBBExerciseTemplateID != "" {
			bbbID, err := FindBBBExerciseTemplateID(ctx, client, lift)
			if err != nil {
				return fmt.Errorf("finding BBB exercise template for %s: %w", lift.DisplayName(), err)
			}
			liftCfg.BBBExerciseTemplateID = bbbID
			fmt.Printf("%s BBB: exercise template ID → %s\n", lift.DisplayName(), bbbID)
		}

		for i, aux := range liftCfg.Warmup {
			if aux.Name == "" {
				continue
			}
			id, err := findTemplateByTitle(ctx, client, aux.Name)
			if err != nil {
				return fmt.Errorf("finding warmup template %q for %s: %w", aux.Name, lift.DisplayName(), err)
			}
			liftCfg.Warmup[i].ExerciseTemplateID = id
			fmt.Printf("%s warmup %q: exercise template ID → %s\n", lift.DisplayName(), aux.Name, id)
		}

		for i, aux := range liftCfg.AuxiliaryExercises {
			if aux.Name == "" {
				continue
			}
			id, err := findTemplateByTitle(ctx, client, aux.Name)
			if err != nil {
				return fmt.Errorf("finding auxiliary template %q for %s: %w", aux.Name, lift.DisplayName(), err)
			}
			liftCfg.AuxiliaryExercises[i].ExerciseTemplateID = id
			fmt.Printf("%s auxiliary %q: exercise template ID → %s\n", lift.DisplayName(), aux.Name, id)
		}

		for i, c := range liftCfg.Cooldown {
			if c.Name == "" {
				continue
			}
			id, err := findTemplateByTitle(ctx, client, c.Name)
			if err != nil {
				return fmt.Errorf("finding cooldown template %q for %s: %w", c.Name, lift.DisplayName(), err)
			}
			liftCfg.Cooldown[i].ExerciseTemplateID = id
			fmt.Printf("%s cooldown %q: exercise template ID → %s\n", lift.DisplayName(), c.Name, id)
		}

		cfg.Lifts[lift] = liftCfg
	}

	for i, w := range cfg.Warmup {
		if w.Name == "" {
			continue
		}
		id, err := findTemplateByTitle(ctx, client, w.Name)
		if err != nil {
			return fmt.Errorf("finding global warmup template %q: %w", w.Name, err)
		}
		cfg.Warmup[i].ExerciseTemplateID = id
		fmt.Printf("global warmup %q: exercise template ID → %s\n", w.Name, id)
	}

	return nil
}

// SaveConfig writes a Config to a JSON file.
func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
