package hevy

import "time"

// SetType represents the type of a set.
type SetType string

const (
	SetTypeWarmup  SetType = "warmup"
	SetTypeNormal  SetType = "normal"
	SetTypeFailure SetType = "failure"
	SetTypeDropset SetType = "dropset"
)

// ExerciseType represents the type of an exercise.
type ExerciseType string

const (
	ExerciseTypeWeightReps             ExerciseType = "weight_reps"
	ExerciseTypeRepsOnly               ExerciseType = "reps_only"
	ExerciseTypeBodyweightReps         ExerciseType = "bodyweight_reps"
	ExerciseTypeBodyweightAssistedReps ExerciseType = "bodyweight_assisted_reps"
	ExerciseTypeDuration               ExerciseType = "duration"
	ExerciseTypeWeightDuration         ExerciseType = "weight_duration"
	ExerciseTypeDistanceDuration       ExerciseType = "distance_duration"
	ExerciseTypeShortDistanceWeight    ExerciseType = "short_distance_weight"
)

// EquipmentCategory represents the equipment used for an exercise.
type EquipmentCategory string

const (
	EquipmentNone           EquipmentCategory = "none"
	EquipmentBarbell        EquipmentCategory = "barbell"
	EquipmentDumbbell       EquipmentCategory = "dumbbell"
	EquipmentKettlebell     EquipmentCategory = "kettlebell"
	EquipmentMachine        EquipmentCategory = "machine"
	EquipmentPlate          EquipmentCategory = "plate"
	EquipmentResistanceBand EquipmentCategory = "resistance_band"
	EquipmentSuspension     EquipmentCategory = "suspension"
	EquipmentOther          EquipmentCategory = "other"
)

// MuscleGroup represents a muscle group.
type MuscleGroup string

const (
	MuscleAbdominals MuscleGroup = "abdominals"
	MuscleShoulders  MuscleGroup = "shoulders"
	MuscleBiceps     MuscleGroup = "biceps"
	MuscleTriceps    MuscleGroup = "triceps"
	MuscleForearms   MuscleGroup = "forearms"
	MuscleQuadriceps MuscleGroup = "quadriceps"
	MuscleHamstrings MuscleGroup = "hamstrings"
	MuscleCalves     MuscleGroup = "calves"
	MuscleGlutes     MuscleGroup = "glutes"
	MuscleAbductors  MuscleGroup = "abductors"
	MuscleAdductors  MuscleGroup = "adductors"
	MuscleLats       MuscleGroup = "lats"
	MuscleUpperBack  MuscleGroup = "upper_back"
	MuscleTraps      MuscleGroup = "traps"
	MuscleLowerBack  MuscleGroup = "lower_back"
	MuscleChest      MuscleGroup = "chest"
	MuscleCardio     MuscleGroup = "cardio"
	MuscleNeck       MuscleGroup = "neck"
	MuscleFullBody   MuscleGroup = "full_body"
	MuscleOther      MuscleGroup = "other"
)

// EventType represents the type of a workout event.
type EventType string

const (
	EventTypeUpdated EventType = "updated"
	EventTypeDeleted EventType = "deleted"
)

// --- Response models ---

type Workout struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	RoutineID   *string           `json:"routine_id"`
	Description string            `json:"description"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     time.Time         `json:"end_time"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CreatedAt   time.Time         `json:"created_at"`
	IsPrivate   bool              `json:"is_private"`
	Exercises   []WorkoutExercise `json:"exercises"`
}

type WorkoutExercise struct {
	Index              int          `json:"index"`
	Title              string       `json:"title"`
	Notes              string       `json:"notes"`
	ExerciseTemplateID string       `json:"exercise_template_id"`
	SupersetID         *int         `json:"supersets_id"`
	Sets               []WorkoutSet `json:"sets"`
}

type WorkoutSet struct {
	Index           int      `json:"index"`
	Type            SetType  `json:"type"`
	WeightKg        *float64 `json:"weight_kg"`
	Reps            *int     `json:"reps"`
	DistanceMeters  *float64 `json:"distance_meters"`
	DurationSeconds *int     `json:"duration_seconds"`
	RPE             *float64 `json:"rpe"`
	CustomMetric    *float64 `json:"custom_metric"`
}

type Routine struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	FolderID  *int              `json:"folder_id"`
	Notes     string            `json:"notes"`
	UpdatedAt time.Time         `json:"updated_at"`
	CreatedAt time.Time         `json:"created_at"`
	Exercises []RoutineExercise `json:"exercises"`
}

type RoutineExercise struct {
	Index              int          `json:"index"`
	Title              string       `json:"title"`
	RestSeconds        *int         `json:"rest_seconds"`
	Notes              string       `json:"notes"`
	ExerciseTemplateID string       `json:"exercise_template_id"`
	SupersetID         *int         `json:"supersets_id"`
	Sets               []RoutineSet `json:"sets"`
}

type RoutineSet struct {
	Index           int       `json:"index"`
	Type            SetType   `json:"type"`
	WeightKg        *float64  `json:"weight_kg"`
	Reps            *int      `json:"reps"`
	RepRange        *RepRange `json:"rep_range"`
	DistanceMeters  *float64  `json:"distance_meters"`
	DurationSeconds *int      `json:"duration_seconds"`
	RPE             *float64  `json:"rpe"`
	CustomMetric    *float64  `json:"custom_metric"`
}

type RepRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type ExerciseTemplate struct {
	ID                    string        `json:"id"`
	Title                 string        `json:"title"`
	Type                  ExerciseType  `json:"type"`
	PrimaryMuscleGroup    MuscleGroup   `json:"primary_muscle_group"`
	SecondaryMuscleGroups []MuscleGroup `json:"secondary_muscle_groups"`
	IsCustom              bool          `json:"is_custom"`
}

type RoutineFolder struct {
	ID        int       `json:"id"`
	Index     int       `json:"index"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

type UserInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type WorkoutEvent struct {
	Type      EventType `json:"type"`
	ID        string    `json:"id"`
	DeletedAt string    `json:"deleted_at,omitempty"`
	Workout   *Workout  `json:"workout,omitempty"`
}

type ExerciseHistoryEntry struct {
	WorkoutID          string  `json:"workout_id"`
	WorkoutTitle       string  `json:"workout_title"`
	WorkoutStartTime   string  `json:"workout_start_time"`
	WorkoutEndTime     string  `json:"workout_end_time"`
	ExerciseTemplateID string  `json:"exercise_template_id"`
	WeightKg           *float64 `json:"weight_kg"`
	Reps               *int     `json:"reps"`
	DistanceMeters     *float64 `json:"distance_meters"`
	DurationSeconds    *int     `json:"duration_seconds"`
	RPE                *float64 `json:"rpe"`
	CustomMetric       *float64 `json:"custom_metric"`
	SetType            SetType  `json:"set_type"`
}

// --- Request models ---

type WorkoutRequest struct {
	Title       string                   `json:"title"`
	Description *string                  `json:"description"`
	StartTime   string                   `json:"start_time"`
	EndTime     string                   `json:"end_time"`
	IsPrivate   bool                     `json:"is_private"`
	Exercises   []WorkoutExerciseRequest `json:"exercises"`
}

type WorkoutExerciseRequest struct {
	ExerciseTemplateID string             `json:"exercise_template_id"`
	SupersetID         *int               `json:"superset_id,omitempty"`
	Notes              string             `json:"notes,omitempty"`
	Sets               []WorkoutSetRequest `json:"sets"`
}

type WorkoutSetRequest struct {
	Type            SetType  `json:"type"`
	WeightKg        *float64 `json:"weight_kg,omitempty"`
	Reps            *int     `json:"reps,omitempty"`
	DistanceMeters  *int     `json:"distance_meters,omitempty"`
	DurationSeconds *int     `json:"duration_seconds,omitempty"`
	CustomMetric    *float64 `json:"custom_metric,omitempty"`
	RPE             *float64 `json:"rpe,omitempty"`
}

type RoutineRequest struct {
	Title     string                   `json:"title"`
	FolderID  *int                     `json:"folder_id,omitempty"`
	Notes     string                   `json:"notes"`
	Exercises []RoutineExerciseRequest `json:"exercises"`
}

type RoutineExerciseRequest struct {
	ExerciseTemplateID string              `json:"exercise_template_id"`
	SupersetID         *int                `json:"superset_id,omitempty"`
	RestSeconds        *int                `json:"rest_seconds,omitempty"`
	Notes              string              `json:"notes,omitempty"`
	Sets               []RoutineSetRequest `json:"sets"`
}

type RoutineSetRequest struct {
	Type            SetType   `json:"type"`
	WeightKg        *float64  `json:"weight_kg,omitempty"`
	Reps            *int      `json:"reps,omitempty"`
	RepRange        *RepRange `json:"rep_range,omitempty"`
	DistanceMeters  *int      `json:"distance_meters,omitempty"`
	DurationSeconds *int      `json:"duration_seconds,omitempty"`
	CustomMetric    *float64  `json:"custom_metric,omitempty"`
}

type ExerciseTemplateRequest struct {
	Title             string            `json:"title"`
	ExerciseType      ExerciseType      `json:"exercise_type"`
	EquipmentCategory EquipmentCategory `json:"equipment_category"`
	MuscleGroup       MuscleGroup       `json:"muscle_group"`
	OtherMuscles      []MuscleGroup     `json:"other_muscles,omitempty"`
}

type RoutineFolderRequest struct {
	Title string `json:"title"`
}

// --- Internal paginated response wrappers ---

type workoutsResponse struct {
	Page      int       `json:"page"`
	PageCount int       `json:"page_count"`
	Workouts  []Workout `json:"workouts"`
}

type workoutCountResponse struct {
	WorkoutCount int `json:"workout_count"`
}

type workoutEventsResponse struct {
	Page      int            `json:"page"`
	PageCount int            `json:"page_count"`
	Events    []WorkoutEvent `json:"events"`
}

type routinesResponse struct {
	Page      int       `json:"page"`
	PageCount int       `json:"page_count"`
	Routines  []Routine `json:"routines"`
}

type singleRoutineResponse struct {
	Routine Routine `json:"routine"`
}

type exerciseTemplatesResponse struct {
	Page              int                `json:"page"`
	PageCount         int                `json:"page_count"`
	ExerciseTemplates []ExerciseTemplate `json:"exercise_templates"`
}

type routineFoldersResponse struct {
	Page           int             `json:"page"`
	PageCount      int             `json:"page_count"`
	RoutineFolders []RoutineFolder `json:"routine_folders"`
}

type userInfoResponse struct {
	Data UserInfo `json:"data"`
}

type exerciseHistoryResponse struct {
	ExerciseHistory []ExerciseHistoryEntry `json:"exercise_history"`
}

type createExerciseTemplateResponse struct {
	ID string `json:"id"`
}
