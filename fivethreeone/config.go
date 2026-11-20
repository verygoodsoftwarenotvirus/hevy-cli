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

// ParseLift resolves a user-supplied string to one of the four main lifts. It
// accepts the canonical key (e.g. "bench_press"), common spellings with spaces
// or hyphens ("bench press", "overhead-press"), and a few short aliases
// ("bench", "ohp", "press", "dead"). Matching is case-insensitive.
func ParseLift(s string) (Lift, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "squat", "sq":
		return Squat, true
	case "bench_press", "bench press", "bench-press", "bench", "bp":
		return BenchPress, true
	case "overhead_press", "overhead press", "overhead-press", "overhead", "ohp", "press", "ohp_press":
		return OverheadPress, true
	case "deadlift", "dead lift", "dead-lift", "dead", "dl":
		return Deadlift, true
	default:
		return "", false
	}
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

// FSLSetCount returns how many First Set Last supplemental sets this lift runs by
// default. The deadlift is capped below the others: its FSL sets sit on top of a lift
// that already taxes recovery hardest, so it takes fewer of them. LiftConfig.FSLSetsByWeek
// overrides it week by week.
func (l Lift) FSLSetCount() int {
	if l == Deadlift {
		return fslDeadliftSetCount
	}
	return fslSetCount
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

// kgPerLb is the international avoirdupois pound, used to convert weights configured in
// pounds (weight_lb) to the kilograms Hevy's API takes.
const kgPerLb = 0.45359237

// AuxiliarySet prescribes one set of an auxiliary exercise, so a movement can ramp load
// and reps across its sets instead of repeating one prescription. Empty fields fall back
// to the parent AuxiliaryExercise's weight, Reps, RepRangeEnd, and DurationSeconds,
// which keeps a scheme that only varies weight from having to restate the reps on every
// rung.
//
// The weight is given in either kilograms (WeightKg) or pounds (WeightLb), never both;
// pounds are converted to kilograms, unrounded, when the routine is built, so a machine
// loaded in pounds shows its exact pound figure in Hevy.
//
// DistanceMeters makes the set a carried or dragged distance rather than a rep count: it
// is sent with its weight and distance and no reps. Hevy takes whole meters.
type AuxiliarySet struct {
	DurationSeconds *int     `json:"duration_seconds,omitempty"`
	DistanceMeters  *int     `json:"distance_meters,omitempty"`
	WeightKg        *float64 `json:"weight_kg,omitempty"`
	WeightLb        *float64 `json:"weight_lb,omitempty"`
	Reps            int      `json:"reps,omitempty"`
	RepRangeEnd     int      `json:"rep_range_end,omitempty"`
}

// weightInKg returns the set's own weight in kilograms, whichever unit it was given in,
// or nil when it has none.
func (s *AuxiliarySet) weightInKg() *float64 {
	return weightInKg(s.WeightKg, s.WeightLb)
}

// weightInKg resolves a kilogram/pound pair to kilograms, preferring kilograms.
func weightInKg(kg, lb *float64) *float64 {
	switch {
	case kg != nil:
		return kg
	case lb != nil:
		converted := *lb * kgPerLb
		return &converted
	default:
		return nil
	}
}

// AuxiliaryExercise describes a user-supplied accessory movement appended to a lift's
// routine on every week (including deload, unless SkipDeload is set). One of Reps,
// DurationSeconds, or DistanceMeters should be set. Weight, rest, and notes are optional;
// leaving the weight out prescribes none, for work whose load is picked on the day.
//
// DistanceMeters prescribes each set as a distance (loaded carries, sled work) instead of
// reps, for Hevy's weight-and-distance exercise types.
//
// WeightPlaceholder names a weight that hasn't been decided yet (e.g. "LEG_CURL_KG"). The
// exercise is generated with no weight until the placeholder is replaced by weight_kg or
// weight_lb; Config.WeightPlaceholders lists the ones still open.
//
// RepRangeEnd turns Reps into the bottom of a rep range (Reps 8, RepRangeEnd 10 is 8-10).
//
// WeightLb is WeightKg given in pounds, for equipment loaded in pounds; set one or the
// other. The unit is per exercise (and per set, in a SetScheme).
//
// WarmupSets are logged as warmup-type sets ahead of the exercise's working sets. Unlike
// SetScheme entries they inherit nothing from the exercise, so the working weight stays a
// single value to edit.
//
// Superset groups the exercise with its neighbors: adjacent exercises in the same list
// that share a Superset label are sent to Hevy as one superset. A label must cover at
// least two exercises, all next to each other.
//
// SetScheme prescribes each set individually and, when present, replaces Sets/Reps/WeightKg
// as the source of the exercise's sets. Use it for ramped accessories (ascending weight,
// descending reps); leave it empty for the common case of N identical sets.
//
// When ExerciseTemplateID is empty, 'hevy 531 fix-exercises' resolves it from Name: it
// looks the title up in the Hevy library and, if it isn't found and ExerciseType is set,
// creates a custom template using ExerciseType/EquipmentCategory/MuscleGroup/OtherMuscles.
type AuxiliaryExercise struct {
	DurationSeconds    *int                   `json:"duration_seconds,omitempty"`
	DistanceMeters     *int                   `json:"distance_meters,omitempty"`
	WeightKg           *float64               `json:"weight_kg,omitempty"`
	WeightLb           *float64               `json:"weight_lb,omitempty"`
	RestSeconds        *int                   `json:"rest_seconds,omitempty"`
	WeightPlaceholder  string                 `json:"weight_placeholder,omitempty"`
	Name               string                 `json:"name,omitempty"`
	Notes              string                 `json:"notes,omitempty"`
	Superset           string                 `json:"superset,omitempty"`
	ExerciseTemplateID string                 `json:"exercise_template_id"`
	ExerciseType       hevy.ExerciseType      `json:"exercise_type,omitempty"`
	EquipmentCategory  hevy.EquipmentCategory `json:"equipment_category,omitempty"`
	MuscleGroup        hevy.MuscleGroup       `json:"muscle_group,omitempty"`
	OtherMuscles       []hevy.MuscleGroup     `json:"other_muscles,omitempty"`
	SetScheme          []AuxiliarySet         `json:"set_scheme,omitempty"`
	WarmupSets         []AuxiliarySet         `json:"warmup_sets,omitempty"`
	Sets               int                    `json:"sets"`
	Reps               int                    `json:"reps,omitempty"`
	RepRangeEnd        int                    `json:"rep_range_end,omitempty"`
	// SkipDeload leaves the exercise out of the deload week's routine.
	SkipDeload bool `json:"skip_deload,omitempty"`
}

// InWeek reports whether the exercise belongs in the given week's routine.
func (a *AuxiliaryExercise) InWeek(week int) bool {
	return week != DeloadWeek || !a.SkipDeload
}

// PrescribedSets returns the exercise's working sets as a SetScheme, expanding the
// Sets/Reps/WeightKg shorthand into one entry per set when no explicit scheme is
// configured. Callers that need to reason about individual sets can then treat both
// spellings the same way. Weights come back in kilograms only: a weight given in pounds
// is converted, and WeightLb is left unset.
func (a *AuxiliaryExercise) PrescribedSets() []AuxiliarySet {
	exerciseKg := weightInKg(a.WeightKg, a.WeightLb)
	if len(a.SetScheme) > 0 {
		// Filled in on a copy, so the configured scheme keeps its own (possibly empty or
		// pound) fields.
		scheme := make([]AuxiliarySet, len(a.SetScheme))
		copy(scheme, a.SetScheme)
		for i := range scheme {
			s := &scheme[i]
			s.WeightKg, s.WeightLb = s.weightInKg(), nil
			if s.WeightKg == nil {
				s.WeightKg = exerciseKg
			}
			if s.DurationSeconds == nil {
				s.DurationSeconds = a.DurationSeconds
			}
			if s.DistanceMeters == nil {
				s.DistanceMeters = a.DistanceMeters
			}
			if s.Reps == 0 {
				s.Reps = a.Reps
			}
			if s.RepRangeEnd == 0 {
				s.RepRangeEnd = a.RepRangeEnd
			}
		}
		return scheme
	}

	scheme := make([]AuxiliarySet, 0, a.Sets)
	for range a.Sets {
		scheme = append(scheme, AuxiliarySet{
			DurationSeconds: a.DurationSeconds,
			DistanceMeters:  a.DistanceMeters,
			WeightKg:        exerciseKg,
			Reps:            a.Reps,
			RepRangeEnd:     a.RepRangeEnd,
		})
	}
	return scheme
}

// PrescribedWarmupSets returns the exercise's warmup sets with their weights in
// kilograms. Each is used exactly as written, with no fallback to the exercise's fields.
func (a *AuxiliaryExercise) PrescribedWarmupSets() []AuxiliarySet {
	warmups := make([]AuxiliarySet, len(a.WarmupSets))
	copy(warmups, a.WarmupSets)
	for i := range warmups {
		warmups[i].WeightKg, warmups[i].WeightLb = warmups[i].weightInKg(), nil
	}
	return warmups
}

// validate rejects an exercise that gives a weight in both units, on itself or any set, a
// weight placeholder alongside the weight it stands in for, or a distance below one meter.
func (a *AuxiliaryExercise) validate() error {
	if a.WeightKg != nil && a.WeightLb != nil {
		return fmt.Errorf("%q: set weight_kg or weight_lb, not both", a.Name)
	}
	if a.WeightPlaceholder != "" && (a.WeightKg != nil || a.WeightLb != nil) {
		return fmt.Errorf("%q: weight_placeholder %q is still set alongside a weight; remove it", a.Name, a.WeightPlaceholder)
	}
	if a.DistanceMeters != nil && *a.DistanceMeters < 1 {
		return fmt.Errorf("%q: distance_meters must be at least 1, got %d", a.Name, *a.DistanceMeters)
	}
	for _, group := range [][]AuxiliarySet{a.SetScheme, a.WarmupSets} {
		for i := range group {
			if group[i].WeightKg != nil && group[i].WeightLb != nil {
				return fmt.Errorf("%q set %d: set weight_kg or weight_lb, not both", a.Name, i+1)
			}
			if d := group[i].DistanceMeters; d != nil && *d < 1 {
				return fmt.Errorf("%q set %d: distance_meters must be at least 1, got %d", a.Name, i+1, *d)
			}
		}
	}
	return nil
}

// LiftConfig holds the training max and exercise template IDs for one lift.
type LiftConfig struct {
	ExerciseTemplateID string `json:"exercise_template_id"`
	// BBBExerciseTemplateID is the separate exercise the BBB preset logs its sets against.
	// It is kept even while another preset is selected, so switching back to BBB doesn't
	// require re-resolving it.
	BBBExerciseTemplateID string `json:"bbb_exercise_template_id"`
	// Assistance overrides the program-wide supplemental preset for this lift alone. An
	// empty value inherits Config.Assistance; set it with 'hevy 531 assistance <preset>
	// --lift=<lift>' and clear it with --clear. This is how a single lift runs something
	// different (e.g. paused FSL on the squat while everything else runs plain FSL)
	// without the program committing to it.
	Assistance AssistanceScheme `json:"assistance,omitempty"`
	// AMRAPNote and AssistanceNote annotate the AMRAP set and the supplemental sets.
	// Hevy's routine sets have no notes of their own, so both land in the notes of the
	// exercise those sets are logged against, and only in weeks that have such sets.
	AMRAPNote      string `json:"amrap_note,omitempty"`
	AssistanceNote string `json:"assistance_note,omitempty"`
	// FSLSetsByWeek sets how many First Set Last sets (plain or paused) the lift runs in
	// each working week, keyed by week number (1-3), so the supplemental volume can taper
	// across the cycle. A week left out uses Lift.FSLSetCount. BBB is unaffected.
	FSLSetsByWeek      map[int]int         `json:"fsl_sets_by_week,omitempty"`
	Warmup             []AuxiliaryExercise `json:"warmup,omitempty"`
	AuxiliaryExercises []AuxiliaryExercise `json:"auxiliary_exercises,omitempty"`
	Cooldown           []AuxiliaryExercise `json:"cooldown,omitempty"`
	TrainingMaxKg      float64             `json:"training_max_kg"`
	UseLbs             bool                `json:"use_lbs,omitempty"`
	// EmptyBarWarmup prepends an empty-barbell warmup set to the main lift's working
	// sets. Enabled for the major lifts except the deadlift.
	EmptyBarWarmup bool `json:"empty_bar_warmup,omitempty"`
	// Disabled drops the lift from routine generation while keeping its training max,
	// template IDs, and accessories in the config, so it can be switched back on later.
	Disabled bool `json:"disabled,omitempty"`
}

// FSLSetsFor returns how many FSL sets lift runs in week: the configured FSLSetsByWeek
// entry, or the lift's default when that week isn't configured.
func (c *LiftConfig) FSLSetsFor(lift Lift, week int) int {
	if n, ok := c.FSLSetsByWeek[week]; ok {
		return n
	}
	return lift.FSLSetCount()
}

// validate rejects FSL set counts for weeks without FSL, or counts below one (use the
// "none" assistance preset to drop supplemental work), and accessories with a weight in
// both units.
func (c *LiftConfig) validate() error {
	for week, n := range c.FSLSetsByWeek {
		if week < 1 || week >= DeloadWeek {
			return fmt.Errorf("fsl_sets_by_week: week %d has no FSL sets (use weeks 1-%d)", week, DeloadWeek-1)
		}
		if n < 1 {
			return fmt.Errorf("fsl_sets_by_week: week %d needs at least 1 set, got %d", week, n)
		}
	}
	return validateAuxList(c.Warmup, c.AuxiliaryExercises, c.Cooldown)
}

// validateAuxList validates every exercise in the given lists, and each list's superset
// labels.
func validateAuxList(lists ...[]AuxiliaryExercise) error {
	for _, list := range lists {
		for i := range list {
			if err := list[i].validate(); err != nil {
				return err
			}
		}
		if err := validateSupersets(list); err != nil {
			return err
		}
	}
	return nil
}

// validateSupersets rejects a superset label that covers only one exercise, or whose
// exercises aren't all adjacent in the list.
func validateSupersets(list []AuxiliaryExercise) error {
	closed := make(map[string]bool)
	for i := 0; i < len(list); {
		label := list[i].Superset
		j := i + 1
		for j < len(list) && list[j].Superset == label {
			j++
		}
		if label != "" {
			switch {
			case closed[label]:
				return fmt.Errorf("superset %q: its exercises must be next to each other", label)
			case j-i < 2:
				return fmt.Errorf("superset %q: needs at least two exercises", label)
			}
			closed[label] = true
		}
		i = j
	}
	return nil
}

// Day is one workout of the training week: either a 5/3/1 main lift (Lift set) or a
// custom day of fixed exercises (Name and Exercises set) that runs unchanged every week
// of the cycle, apart from exercises marked SkipDeload. A custom day's Warmup precedes
// its exercises; when empty, the program-wide Config.Warmup is used instead. Main-lift
// days take their warmup from the lift's LiftConfig.Warmup, so Warmup must be empty there.
type Day struct {
	Lift      Lift                `json:"lift,omitempty"`
	Name      string              `json:"name,omitempty"`
	Warmup    []AuxiliaryExercise `json:"warmup,omitempty"`
	Exercises []AuxiliaryExercise `json:"exercises,omitempty"`
}

// IsCustom reports whether the day is a custom day rather than a main-lift day.
func (d *Day) IsCustom() bool {
	return d.Lift == ""
}

// DisplayName returns the name the day's routines are titled with.
func (d *Day) DisplayName() string {
	if d.IsCustom() {
		return d.Name
	}
	return d.Lift.DisplayName()
}

// Config holds the complete 5/3/1 program state.
type Config struct {
	Lifts      map[Lift]LiftConfig     `json:"lifts"`
	RoutineIDs map[Lift]map[int]string `json:"routine_ids,omitempty"`
	// CustomRoutineIDs is RoutineIDs for custom days, keyed by the day's name.
	CustomRoutineIDs map[string]map[int]string `json:"custom_routine_ids,omitempty"`
	FolderID         *int                      `json:"folder_id,omitempty"`
	// Assistance selects the supplemental-volume preset applied to every lift. An empty
	// value means DefaultAssistanceScheme; switch presets with 'hevy 531 assistance'.
	Assistance AssistanceScheme    `json:"assistance,omitempty"`
	Warmup     []AuxiliaryExercise `json:"warmup,omitempty"`
	// Days orders the training week. Leaving it out means one day per configured lift,
	// in AllLifts order. A disabled lift's day is skipped wherever it is listed, so
	// re-enabling the lift puts it straight back in its slot.
	Days        []Day `json:"days,omitempty"`
	CycleNumber int   `json:"cycle_number"`
}

// TrainingDays returns the days routines are generated for, in order: Days (or one day
// per lift when Days is empty), minus days for lifts that are disabled or not configured.
func (c *Config) TrainingDays() []Day {
	days := c.Days
	if len(days) == 0 {
		for _, lift := range AllLifts() {
			days = append(days, Day{Lift: lift})
		}
	}

	var active []Day
	for i := range days {
		if !days[i].IsCustom() {
			liftCfg, ok := c.Lifts[days[i].Lift]
			if !ok || liftCfg.Disabled {
				continue
			}
		}
		active = append(active, days[i])
	}
	return active
}

// WeightPlaceholder is one exercise whose weight is still a placeholder.
type WeightPlaceholder struct {
	// Day is the display name of the training day the exercise is on.
	Day string
	// Exercise is the exercise's configured name.
	Exercise string
	// Placeholder is the name the open weight was given in the config.
	Placeholder string
}

// WeightPlaceholders lists every exercise on a training day whose weight is still a
// placeholder, in routine order. Those exercises are generated with no weight.
func (c *Config) WeightPlaceholders() []WeightPlaceholder {
	var open []WeightPlaceholder
	days := c.TrainingDays()
	for i := range days {
		day := &days[i]
		lists := [][]AuxiliaryExercise{day.Warmup, day.Exercises}
		if !day.IsCustom() {
			liftCfg := c.Lifts[day.Lift]
			lists = [][]AuxiliaryExercise{liftCfg.Warmup, liftCfg.AuxiliaryExercises, liftCfg.Cooldown}
		}
		for _, list := range lists {
			for j := range list {
				if list[j].WeightPlaceholder != "" {
					open = append(open, WeightPlaceholder{
						Day:         day.DisplayName(),
						Exercise:    list[j].Name,
						Placeholder: list[j].WeightPlaceholder,
					})
				}
			}
		}
	}
	return open
}

// validate rejects Days entries that would otherwise be silently skipped or produce an
// untitled routine, and lift or exercise settings that can't be applied as written.
func (c *Config) validate() error {
	for _, lift := range AllLifts() {
		liftCfg, ok := c.Lifts[lift]
		if !ok {
			continue
		}
		if err := liftCfg.validate(); err != nil {
			return fmt.Errorf("%s: %w", lift.DisplayName(), err)
		}
	}
	if err := validateAuxList(c.Warmup); err != nil {
		return fmt.Errorf("global warmup: %w", err)
	}

	seen := make(map[string]bool)
	for i := range c.Days {
		day := &c.Days[i]
		switch {
		case day.Lift != "" && day.Name != "":
			return fmt.Errorf("day %d: set either lift or name, not both", i+1)
		case day.IsCustom() && day.Name == "":
			return fmt.Errorf("day %d: a custom day needs a name", i+1)
		case !day.IsCustom():
			if _, ok := c.Lifts[day.Lift]; !ok {
				return fmt.Errorf("day %d: lift %q is not in the config", i+1, day.Lift)
			}
			if len(day.Warmup) > 0 {
				return fmt.Errorf("day %d: set %s's warmup under lifts, not on the day", i+1, day.Lift.DisplayName())
			}
		}
		if seen[day.DisplayName()] {
			return fmt.Errorf("day %d: %q is listed more than once", i+1, day.DisplayName())
		}
		seen[day.DisplayName()] = true
		if err := validateAuxList(day.Warmup, day.Exercises); err != nil {
			return fmt.Errorf("day %d: %w", i+1, err)
		}
	}
	return nil
}

// AssistanceScheme returns the configured supplemental-volume preset, falling back to
// DefaultAssistanceScheme when the config doesn't name one.
func (c *Config) AssistanceScheme() AssistanceScheme {
	if c.Assistance == "" {
		return DefaultAssistanceScheme
	}
	return c.Assistance
}

// AssistanceSchemeFor returns the supplemental preset in effect for one lift: the lift's
// own override when it has one, and the program-wide preset otherwise.
func (c *Config) AssistanceSchemeFor(lift Lift) AssistanceScheme {
	if liftCfg, ok := c.Lifts[lift]; ok && liftCfg.Assistance != "" {
		return liftCfg.Assistance
	}
	return c.AssistanceScheme()
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
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
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

	for i := range cfg.Days {
		if err := resolveAuxTemplates(ctx, client, cfg.Days[i].Warmup, cfg.Days[i].Name+" day warmup"); err != nil {
			return err
		}
		if err := resolveAuxTemplates(ctx, client, cfg.Days[i].Exercises, cfg.Days[i].Name+" day"); err != nil {
			return err
		}
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
