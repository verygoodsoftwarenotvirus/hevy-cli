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

// TMIncrementKg returns the standard 5/3/1 training-max increase applied when
// advancing to the next cycle: 2.5 kg for upper-body lifts (bench, overhead
// press) and 5 kg for lower-body lifts (squat, deadlift).
func (l Lift) TMIncrementKg() float64 {
	if l.IsUpperBody() {
		return 2.5
	}
	return 5.0
}

// AuxiliaryExercise describes a user-supplied accessory movement appended to a lift's
// routine on every week (including deload). Either Reps or DurationSeconds should be set.
// Weight, rest, and notes are optional.
//
// When ExerciseTemplateID is empty, 'hevy 531 fix-exercises' resolves it from Name: it
// looks the title up in the Hevy library and, if it isn't found and ExerciseType is set,
// creates a custom template using ExerciseType/EquipmentCategory/MuscleGroup/OtherMuscles.
type AuxiliaryExercise struct {
	DurationSeconds    *int                   `json:"duration_seconds,omitempty"`
	WeightKg           *float64               `json:"weight_kg,omitempty"`
	RestSeconds        *int                   `json:"rest_seconds,omitempty"`
	Name               string                 `json:"name,omitempty"`
	Notes              string                 `json:"notes,omitempty"`
	ExerciseTemplateID string                 `json:"exercise_template_id"`
	ExerciseType       hevy.ExerciseType      `json:"exercise_type,omitempty"`
	EquipmentCategory  hevy.EquipmentCategory `json:"equipment_category,omitempty"`
	MuscleGroup        hevy.MuscleGroup       `json:"muscle_group,omitempty"`
	OtherMuscles       []hevy.MuscleGroup     `json:"other_muscles,omitempty"`
	Sets               int                    `json:"sets"`
	Reps               int                    `json:"reps,omitempty"`
}

// LiftConfig holds the training max and exercise template IDs for one lift.
type LiftConfig struct {
	ExerciseTemplateID    string              `json:"exercise_template_id"`
	BBBExerciseTemplateID string              `json:"bbb_exercise_template_id"`
	Warmup                []AuxiliaryExercise `json:"warmup,omitempty"`
	AuxiliaryExercises    []AuxiliaryExercise `json:"auxiliary_exercises,omitempty"`
	Cooldown              []AuxiliaryExercise `json:"cooldown,omitempty"`
	TrainingMaxKg         float64             `json:"training_max_kg"`
	UseLbs                bool                `json:"use_lbs,omitempty"`
	// EmptyBarWarmup prepends an empty-barbell warmup set to the main lift's working
	// sets. Enabled for the major lifts except the deadlift.
	EmptyBarWarmup bool `json:"empty_bar_warmup,omitempty"`
}

// Config holds the complete 5/3/1 program state.
type Config struct {
	Lifts       map[Lift]LiftConfig     `json:"lifts"`
	RoutineIDs  map[Lift]map[int]string `json:"routine_ids,omitempty"`
	FolderID    *int                    `json:"folder_id,omitempty"`
	Warmup      []AuxiliaryExercise     `json:"warmup,omitempty"`
	CycleNumber int                     `json:"cycle_number"`
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

// lookupTemplateByTitle searches all exercise templates for one matching the given title
// (case-insensitive). The boolean reports whether a match was found, distinguishing a
// genuine "not found" from an API error so callers can create the template when missing.
func lookupTemplateByTitle(ctx context.Context, client *hevy.Client, title string) (string, bool, error) {
	want := strings.ToLower(title)
	for tmpl, err := range client.ListExerciseTemplates(ctx) {
		if err != nil {
			return "", false, fmt.Errorf("listing exercise templates: %w", err)
		}
		if strings.ToLower(tmpl.Title) == want {
			return tmpl.ID, true, nil
		}
	}
	return "", false, nil
}

// findTemplateByTitle searches all exercise templates for one matching the given title
// (case-insensitive) and returns its ID, erroring if no match exists.
func findTemplateByTitle(ctx context.Context, client *hevy.Client, title string) (string, error) {
	id, found, err := lookupTemplateByTitle(ctx, client, title)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("exercise template %q not found", title)
	}
	return id, nil
}

// resolveAuxTemplates fills in the ExerciseTemplateID of each named exercise that doesn't
// already have one. Names are resolved against the Hevy library; a name that isn't found
// is created as a custom template when the entry carries an ExerciseType (its creation
// metadata), and is otherwise treated as an error. Entries that already have an
// ExerciseTemplateID are left untouched — the Name is only a label there, not necessarily
// an exact Hevy title. desc labels the group for log/error messages.
func resolveAuxTemplates(ctx context.Context, client *hevy.Client, list []AuxiliaryExercise, desc string) error {
	for i := range list {
		aux := &list[i]
		if aux.Name == "" || aux.ExerciseTemplateID != "" {
			continue
		}
		id, found, err := lookupTemplateByTitle(ctx, client, aux.Name)
		if err != nil {
			return fmt.Errorf("finding %s %q: %w", desc, aux.Name, err)
		}
		if !found {
			if aux.ExerciseType == "" {
				return fmt.Errorf("%s %q not found in Hevy and has no exercise_type to create it from", desc, aux.Name)
			}
			id, err = client.CreateExerciseTemplate(ctx, &hevy.ExerciseTemplateRequest{
				Title:             aux.Name,
				ExerciseType:      aux.ExerciseType,
				EquipmentCategory: aux.EquipmentCategory,
				MuscleGroup:       aux.MuscleGroup,
				OtherMuscles:      aux.OtherMuscles,
			})
			if err != nil {
				return fmt.Errorf("creating custom %s %q: %w", desc, aux.Name, err)
			}
			fmt.Printf("%s %q: created custom exercise template → %s\n", desc, aux.Name, id)
		} else {
			fmt.Printf("%s %q: exercise template ID → %s\n", desc, aux.Name, id)
		}
		aux.ExerciseTemplateID = id
	}
	return nil
}

// FindExerciseTemplateID searches for the main lift exercise template.
func FindExerciseTemplateID(ctx context.Context, client *hevy.Client, lift Lift) (string, error) {
	return findTemplateByTitle(ctx, client, lift.HevyTitle())
}

// FindBBBExerciseTemplateID searches for the BBB assistance exercise template.
func FindBBBExerciseTemplateID(ctx context.Context, client *hevy.Client, lift Lift) (string, error) {
	return findTemplateByTitle(ctx, client, lift.HevyBBBTitle())
}

// RefreshExerciseTemplateIDs resolves exercise template IDs in the config from the Hevy
// API. It re-resolves BBB assistance IDs (which have known canonical titles) and fills in
// any warmup/auxiliary/cooldown exercises that have a Name but no ExerciseTemplateID yet,
// creating custom templates for names not present in the Hevy library that carry an
// exercise_type (see AuxiliaryExercise). Entries that already have an ID are left as-is,
// so hand-picked IDs are never clobbered.
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
		name := lift.DisplayName()

		if liftCfg.BBBExerciseTemplateID != "" {
			bbbID, err := FindBBBExerciseTemplateID(ctx, client, lift)
			if err != nil {
				return fmt.Errorf("finding BBB exercise template for %s: %w", name, err)
			}
			liftCfg.BBBExerciseTemplateID = bbbID
			fmt.Printf("%s BBB: exercise template ID → %s\n", name, bbbID)
		}

		if err := resolveAuxTemplates(ctx, client, liftCfg.Warmup, name+" warmup"); err != nil {
			return err
		}
		if err := resolveAuxTemplates(ctx, client, liftCfg.AuxiliaryExercises, name+" auxiliary"); err != nil {
			return err
		}
		if err := resolveAuxTemplates(ctx, client, liftCfg.Cooldown, name+" cooldown"); err != nil {
			return err
		}

		cfg.Lifts[lift] = liftCfg
	}

	return resolveAuxTemplates(ctx, client, cfg.Warmup, "global warmup")
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
