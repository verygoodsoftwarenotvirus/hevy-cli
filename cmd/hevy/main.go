package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
	"github.com/verygoodsoftwarenotvirus/hevy-cli/archive"
	"github.com/verygoodsoftwarenotvirus/hevy-cli/fivethreeone"
	"github.com/verygoodsoftwarenotvirus/hevy-cli/strongimport"
)

// archiveProgressEvery is how many scanned workouts pass between archive progress lines.
const archiveProgressEvery = 25

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	apiKey := os.Getenv("HEVY_API_KEY")
	if apiKey == "" {
		slog.Error("HEVY_API_KEY environment variable is required")
		os.Exit(1)
	}

	client := hevy.NewClient(apiKey)
	ctx := context.Background()

	switch os.Args[1] {
	case "user":
		cmdUser(ctx, client)
	case "exercises":
		cmdExercises(ctx, client, os.Args[2:])
	case "workouts":
		cmdWorkouts(ctx, client, os.Args[2:])
	case "routines":
		cmdRoutines(ctx, client, os.Args[2:])
	case "531":
		cmd531(ctx, client, os.Args[2:])
	case "archive":
		cmdArchive(ctx, apiKey, os.Args[2:])
	case "fix-durations":
		cmdFixDurations(ctx, client, os.Args[2:])
	case "restore-durations":
		cmdRestoreDurations(ctx, client, os.Args[2:])
	case "fill-durations":
		cmdFillDurations(ctx, client, os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: hevy <command> [args]

Commands:
  user                       Print current user info
  exercises list             List all exercise templates
  exercises search <name>    Search exercise templates by name
  exercises get <id>         Get a single exercise template
  workouts list              List recent workouts
  workouts lastweek [-n N]   Print workouts from N weeks ago (default 1, 0 = this week)
  workouts cyclesofar        Print this 5/3/1 cycle's working weeks so far (excludes deload)
  workouts lastcycle         Print the previous 5/3/1 cycle's working weeks (excludes deload)
  workouts count             Print total workout count
  workouts get <id>          Get a single workout
  routines list [--folder=T] List routines, optionally filtered by folder title
  routines get <id>          Get a single routine
  archive --year Y [--out FILE] [--tz ZONE] [--force]
                             Export a year of workouts to a SQLite file
  fix-durations [--apply] [--limit N]
                             Clear the workout-length durations the Strong import
                             stamped onto every set (dry run unless --apply)
  restore-durations --csv FILE [--apply]
                             Restore the real per-set times from a Strong CSV
                             export (dry run unless --apply)
  fill-durations --set NAME=SECS [--set ...] [--apply]
                             Fill blank timed sets with an estimate, noting on
                             each exercise that it is one (dry run unless --apply)
  531 init --config=FILE          Set up 5/3/1 program
  531 sync --config=FILE          Update routines for current week (never changes training maxes)
  531 status --config=FILE        Print current program status
  531 tm <lift> (--set N|--by N|--increment)  Set or adjust a lift's training max
  531 assistance [bbb|fsl|fsl-paused|none] [--lift=LIFT] [--clear]
                                  Show or switch the supplemental-volume preset
  531 fix-exercises --config=FILE Resolve/create warmup & auxiliary exercise templates

Environment:
  HEVY_API_KEY               API key (required, from https://hevy.com/settings?developer)`)
}

func cmdUser(ctx context.Context, client *hevy.Client) {
	info, err := client.GetUserInfo(ctx)
	if err != nil {
		slog.Error("getting user info", "error", err)
		os.Exit(1)
	}
	fmt.Printf("ID:   %s\nName: %s\nURL:  %s\n", info.ID, info.Name, info.URL)
}

// cmdArchive exports a calendar year of workouts into a SQLite file.
//
// It builds its own client rather than reusing main's, because a full-year scan is long enough that
// retrying rate limits and transient failures matters.
func cmdArchive(ctx context.Context, apiKey string, args []string) {
	fs := flag.NewFlagSet("archive", flag.ExitOnError)
	year := fs.Int("year", 0, "calendar year to archive (required)")
	out := fs.String("out", "", "output SQLite file (default: hevy-<year>.db)")
	tz := fs.String("tz", "", "IANA timezone used to decide which year a workout falls in (default: local)")
	force := fs.Bool("force", false, "overwrite the output file if it already exists")
	fs.Parse(args)

	if *year <= 0 {
		fmt.Fprintln(os.Stderr, "--year is required, e.g. hevy archive --year 2025")
		os.Exit(1)
	}

	loc := time.Local //nolint:gosmopolitan // local time is the intended default; --tz overrides it.
	if *tz != "" {
		var err error
		if loc, err = time.LoadLocation(*tz); err != nil {
			slog.Error("loading timezone", "tz", *tz, "error", err)
			os.Exit(1)
		}
	}

	outPath := *out
	if outPath == "" {
		outPath = fmt.Sprintf("hevy-%d.db", *year)
	}

	// Scanning a year means many sequential API pages, so report progress rather than sit silent.
	reportedAt := 0
	progress := func(fetched, kept int) {
		if fetched-reportedAt < archiveProgressEvery {
			return
		}
		reportedAt = fetched
		fmt.Fprintf(os.Stderr, "scanned %d workouts, %d in %d...\n", fetched, kept, *year)
	}

	result, err := archive.Run(ctx, archive.NewClient(apiKey), archive.Options{
		Location: loc,
		Out:      outPath,
		Year:     *year,
		Force:    *force,
	}, progress)
	if err != nil {
		slog.Error("archiving workouts", "error", err)
		os.Exit(1)
	}

	if result.Workouts == 0 {
		fmt.Printf("No workouts found in %d (scanned %d). Wrote an empty archive to %s\n",
			*year, result.Fetched, outPath)
		return
	}

	fmt.Printf("Archived %d workouts (%d exercises, %d sets) spanning %s to %s into %s\n",
		result.Workouts, result.Exercises, result.Sets,
		result.First.Format(time.DateOnly), result.Last.Format(time.DateOnly), outPath)
	fmt.Printf("Scanned %d workouts in total.\n", result.Fetched)

	if result.OutOfOrder {
		fmt.Println("Note: the API returned workouts out of start-time order, so the full history was scanned.")
	}
}

func cmdExercises(ctx context.Context, client *hevy.Client, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hevy exercises <list|search|get> [args]")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		for t, err := range client.ListExerciseTemplates(ctx) {
			if err != nil {
				slog.Error("listing exercises", "error", err)
				os.Exit(1)
			}
			fmt.Printf("%-40s %s (type: %s, muscle: %s)\n", t.ID, t.Title, t.Type, t.PrimaryMuscleGroup)
		}
	case "search":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: hevy exercises search <name>")
			os.Exit(1)
		}
		query := strings.ToLower(strings.Join(args[1:], " "))
		found := 0
		for t, err := range client.ListExerciseTemplates(ctx) {
			if err != nil {
				slog.Error("searching exercises", "error", err)
				os.Exit(1)
			}
			if strings.Contains(strings.ToLower(t.Title), query) {
				fmt.Printf("%-40s %s (type: %s, muscle: %s, custom: %v)\n", t.ID, t.Title, t.Type, t.PrimaryMuscleGroup, t.IsCustom)
				found++
			}
		}
		if found == 0 {
			fmt.Printf("No exercises found matching %q\n", query)
		} else {
			fmt.Printf("\n%d exercise(s) found\n", found)
		}
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: hevy exercises get <id>")
			os.Exit(1)
		}
		t, err := client.GetExerciseTemplate(ctx, args[1])
		if err != nil {
			slog.Error("getting exercise", "error", err)
			os.Exit(1)
		}
		fmt.Printf("ID:        %s\nTitle:     %s\nType:      %s\nMuscle:    %s\nSecondary: %v\nCustom:    %v\n",
			t.ID, t.Title, t.Type, t.PrimaryMuscleGroup, t.SecondaryMuscleGroups, t.IsCustom)
	default:
		fmt.Fprintf(os.Stderr, "unknown exercises command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdWorkouts(ctx context.Context, client *hevy.Client, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hevy workouts <list|lastweek|cyclesofar|lastcycle|count|get> [args]")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		for w, err := range client.ListWorkouts(ctx) {
			if err != nil {
				slog.Error("listing workouts", "error", err)
				os.Exit(1)
			}
			fmt.Printf("%s  %s  (%s — %s)  %d exercises\n",
				w.ID, w.Title,
				w.StartTime.Format("2006-01-02 15:04"),
				w.EndTime.Format("15:04"),
				len(w.Exercises))
		}
	case "lastweek":
		cmdWorkoutsLastWeek(ctx, client, args[1:])
	case "cyclesofar":
		cmdWorkoutsCycle(ctx, client, args[1:], 0)
	case "lastcycle":
		cmdWorkoutsCycle(ctx, client, args[1:], 1)
	case "count":
		count, err := client.GetWorkoutCount(ctx)
		if err != nil {
			slog.Error("getting workout count", "error", err)
			os.Exit(1)
		}
		fmt.Printf("%d workouts\n", count)
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: hevy workouts get <id>")
			os.Exit(1)
		}
		w, err := client.GetWorkout(ctx, args[1])
		if err != nil {
			slog.Error("getting workout", "error", err)
			os.Exit(1)
		}
		fmt.Printf("ID:    %s\nTitle: %s\nDate:  %s — %s\n", w.ID, w.Title,
			w.StartTime.Format("2006-01-02 15:04"), w.EndTime.Format("15:04"))
		printWorkoutExercises(w.Exercises, true)
	default:
		fmt.Fprintf(os.Stderr, "unknown workouts command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdWorkoutsLastWeek(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("workouts lastweek", flag.ExitOnError)
	n := fs.Int("n", 1, "weeks back (0 = current week, 1 = last completed week)")
	fs.Parse(args)

	if *n < 0 {
		fmt.Fprintln(os.Stderr, "-n must be >= 0")
		os.Exit(1)
	}

	// Compute Monday 00:00 local time of the target week (ISO week, Mon start).
	now := time.Now()
	wd := int(now.Weekday()) // Sunday = 0 .. Saturday = 6
	if wd == 0 {
		wd = 7
	}
	daysFromMonday := wd - 1
	thisMonday := time.Date(now.Year(), now.Month(), now.Day()-daysFromMonday, 0, 0, 0, 0, now.Location())
	start := thisMonday.AddDate(0, 0, -7*(*n))
	end := start.AddDate(0, 0, 7)

	var collected []hevy.Workout
	for w, err := range client.ListWorkouts(ctx) {
		if err != nil {
			slog.Error("listing workouts", "error", err)
			os.Exit(1)
		}
		if w.StartTime.Before(start) {
			// Hevy returns workouts newest-first; everything past this point is older than our window.
			break
		}
		if w.StartTime.Before(end) {
			collected = append(collected, w)
		}
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].StartTime.Before(collected[j].StartTime)
	})

	fmt.Printf("Week of %s — %s\n",
		start.Format("Mon 2006-01-02"),
		start.AddDate(0, 0, 6).Format("Mon 2006-01-02"))

	if len(collected) == 0 {
		fmt.Println("No workouts logged in this week.")
		return
	}

	fmt.Printf("%d workout(s)\n", len(collected))

	for _, w := range collected {
		printWorkoutDetail(w)
	}
}

// setDetail is the printable view of a set from either a workout or a routine. Both API
// types carry the same measurements — only routines have rep ranges, and only logged
// workouts have an RPE — so one renderer covers every printout.
type setDetail struct {
	WeightKg        *float64
	Reps            *int
	RepRange        *hevy.RepRange
	DurationSeconds *int
	DistanceMeters  *float64
	RPE             *float64
	Type            hevy.SetType
}

func workoutSetDetail(s *hevy.WorkoutSet) setDetail {
	return setDetail{
		Type:            s.Type,
		WeightKg:        s.WeightKg,
		Reps:            s.Reps,
		DurationSeconds: s.DurationSeconds,
		DistanceMeters:  s.DistanceMeters,
		RPE:             s.RPE,
	}
}

func routineSetDetail(s *hevy.RoutineSet) setDetail {
	return setDetail{
		Type:            s.Type,
		WeightKg:        s.WeightKg,
		Reps:            s.Reps,
		RepRange:        s.RepRange,
		DurationSeconds: s.DurationSeconds,
		DistanceMeters:  s.DistanceMeters,
		RPE:             s.RPE,
	}
}

// String renders a set as an indented line, e.g. "    [normal] 60.0 kg x5 @RPE 8.0".
// Only the measurements the set actually carries are printed, so duration-based work
// (dead hangs, planks) shows its time instead of reps it will never have.
func (d setDetail) String() string {
	parts := []string{fmt.Sprintf("[%s]", d.Type)}
	if d.WeightKg != nil {
		parts = append(parts, fmt.Sprintf("%.1f kg", *d.WeightKg))
	}
	if d.Reps != nil {
		parts = append(parts, fmt.Sprintf("x%d", *d.Reps))
	}
	if d.RepRange != nil {
		if d.RepRange.Start == d.RepRange.End {
			parts = append(parts, fmt.Sprintf("x%d", d.RepRange.Start))
		} else {
			parts = append(parts, fmt.Sprintf("x%d-%d", d.RepRange.Start, d.RepRange.End))
		}
	}
	if d.DistanceMeters != nil {
		parts = append(parts, fmt.Sprintf("%s m", strconv.FormatFloat(*d.DistanceMeters, 'f', -1, 64)))
	}
	if d.DurationSeconds != nil {
		parts = append(parts, formatDuration(*d.DurationSeconds))
	}
	if d.RPE != nil {
		parts = append(parts, fmt.Sprintf("@RPE %.1f", *d.RPE))
	}
	return "    " + strings.Join(parts, " ")
}

// formatDuration renders a set's duration as "45s", "1m", or "1m 30s".
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes, remainder := seconds/60, seconds%60
	if remainder == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm %ds", minutes, remainder)
}

// printWorkoutDetail prints a workout's exercises and sets.
func printWorkoutDetail(w hevy.Workout) {
	fmt.Printf("\n%s — %s\n", w.StartTime.Format("Mon 2006-01-02 15:04"), w.Title)
	printWorkoutExercises(w.Exercises, false)
}

// printWorkoutExercises prints each logged exercise with its notes and sets.
func printWorkoutExercises(exercises []hevy.WorkoutExercise, showTemplateIDs bool) {
	for _, e := range exercises {
		if showTemplateIDs {
			fmt.Printf("\n  %s (%s)\n", e.Title, e.ExerciseTemplateID)
		} else {
			fmt.Printf("\n  %s\n", e.Title)
		}
		if e.Notes != "" {
			fmt.Printf("    Notes: %s\n", e.Notes)
		}
		for i := range e.Sets {
			fmt.Println(workoutSetDetail(&e.Sets[i]))
		}
	}
}

// printRoutineExercises prints each planned exercise with its notes and sets.
func printRoutineExercises(exercises []hevy.RoutineExercise, showTemplateIDs bool) {
	for _, e := range exercises {
		if showTemplateIDs {
			fmt.Printf("\n  %s (%s)\n", e.Title, e.ExerciseTemplateID)
		} else {
			fmt.Printf("\n  %s\n", e.Title)
		}
		if e.Notes != "" {
			fmt.Printf("    Notes: %s\n", e.Notes)
		}
		for i := range e.Sets {
			fmt.Println(routineSetDetail(&e.Sets[i]))
		}
	}
}

// cycleTitleRE matches 5/3/1 workout titles like "C3W1 -- Squat", capturing the
// cycle number and the week number within that cycle.
var cycleTitleRE = regexp.MustCompile(`^C(\d+)W(\d+)`)

// cmdWorkoutsCycle prints the working-week (non-deload) workouts of a 5/3/1
// cycle, identified purely from workout titles (C<cycle>W<week>). cyclesAgo is 0
// for the current cycle ("cyclesofar") and 1 for the previous one ("lastcycle").
func cmdWorkoutsCycle(ctx context.Context, client *hevy.Client, args []string, cyclesAgo int) {
	name := "cyclesofar"
	if cyclesAgo > 0 {
		name = "lastcycle"
	}
	fs := flag.NewFlagSet("workouts "+name, flag.ExitOnError)
	includeDeload := fs.Bool("include-deload", false, "include the deload week (week 4)")
	fs.Parse(args)

	// Hevy returns workouts newest-first. The first title we can parse fixes the
	// current cycle; the target cycle is that minus cyclesAgo. Cycle numbers only
	// ever increase over time, so once we see an older cycle we're done.
	targetCycle := -1
	var collected []hevy.Workout
	for w, err := range client.ListWorkouts(ctx) {
		if err != nil {
			slog.Error("listing workouts", "error", err)
			os.Exit(1)
		}
		m := cycleTitleRE.FindStringSubmatch(w.Title)
		if m == nil {
			continue
		}
		cyc, _ := strconv.Atoi(m[1])
		wk, _ := strconv.Atoi(m[2])

		if targetCycle == -1 {
			targetCycle = cyc - cyclesAgo
		}
		if cyc > targetCycle {
			continue // a newer cycle than the one we want (only happens for lastcycle)
		}
		if cyc < targetCycle {
			break // reached an older cycle; everything below is older still
		}
		if wk >= fivethreeone.DeloadWeek && !*includeDeload {
			continue // skip deload
		}
		collected = append(collected, w)
	}

	if targetCycle < 1 {
		if cyclesAgo > 0 {
			fmt.Println("No previous cycle found.")
		} else {
			fmt.Println("No 5/3/1 workouts found (titles like \"C3W1\").")
		}
		return
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].StartTime.Before(collected[j].StartTime)
	})

	scope := "through today"
	if cyclesAgo > 0 {
		scope = "(previous cycle)"
	}
	fmt.Printf("Cycle %d %s\n", targetCycle, scope)

	if len(collected) == 0 {
		fmt.Println("No working-week workouts logged for this cycle.")
		return
	}

	fmt.Printf("%d workout(s)\n", len(collected))
	for _, w := range collected {
		printWorkoutDetail(w)
	}
}

func cmdRoutines(ctx context.Context, client *hevy.Client, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hevy routines <list|get> [args]")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		cmdRoutinesList(ctx, client, args[1:])
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: hevy routines get <id>")
			os.Exit(1)
		}
		r, err := client.GetRoutine(ctx, args[1])
		if err != nil {
			slog.Error("getting routine", "error", err)
			os.Exit(1)
		}
		fmt.Printf("ID:    %s\nTitle: %s\nNotes: %s\n", r.ID, r.Title, r.Notes)
		printRoutineExercises(r.Exercises, true)
	default:
		fmt.Fprintf(os.Stderr, "unknown routines command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdRoutinesList(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("routines list", flag.ExitOnError)
	folderTitle := fs.String("folder", "", "only list routines in the folder with this title")
	fs.Parse(args)

	var folderID *int
	if *folderTitle != "" {
		for f, err := range client.ListRoutineFolders(ctx) {
			if err != nil {
				slog.Error("listing folders", "error", err)
				os.Exit(1)
			}
			if f.Title == *folderTitle {
				id := f.ID
				folderID = &id
				break
			}
		}
		if folderID == nil {
			fmt.Fprintf(os.Stderr, "no folder found with title %q\n", *folderTitle)
			os.Exit(1)
		}
	}

	var matched []hevy.Routine
	for r, err := range client.ListRoutines(ctx) {
		if err != nil {
			slog.Error("listing routines", "error", err)
			os.Exit(1)
		}
		if folderID != nil && (r.FolderID == nil || *r.FolderID != *folderID) {
			continue
		}
		matched = append(matched, r)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.Before(matched[j].CreatedAt)
	})

	if *folderTitle == "" {
		for _, r := range matched {
			fmt.Printf("%s  %s  (%d exercises)\n", r.ID, r.Title, len(r.Exercises))
		}
		return
	}

	fmt.Printf("Folder: %s\n", *folderTitle)
	if len(matched) == 0 {
		fmt.Println("No routines in this folder.")
		return
	}
	fmt.Printf("%d routine(s)\n", len(matched))

	for _, r := range matched {
		fmt.Printf("\n%s\n", r.Title)
		if r.Notes != "" {
			fmt.Printf("  Notes: %s\n", r.Notes)
		}
		printRoutineExercises(r.Exercises, false)
	}
}

func cmd531(ctx context.Context, client *hevy.Client, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hevy 531 <init|sync|status|tm|assistance|fix-exercises> --config=FILE")
		os.Exit(1)
	}

	switch args[0] {
	case "init":
		cmd531Init(ctx, client, args[1:])
	case "sync":
		cmd531Sync(ctx, client, args[1:])
	case "status":
		cmd531Status(args[1:])
	case "tm":
		cmd531TM(args[1:])
	case "fix-exercises":
		cmd531FixExercises(ctx, client, args[1:])
	case "assistance":
		cmd531Assistance(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown 531 command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmd531Init(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("531 init", flag.ExitOnError)
	configPath := fs.String("config", "531.json", "path to 5/3/1 config file")
	configOnly := fs.Bool("config-only", false, "write config without creating routines in Hevy")
	fs.Parse(args)

	scanner := bufio.NewScanner(os.Stdin)

	cfg := &fivethreeone.Config{
		Lifts:       make(map[fivethreeone.Lift]fivethreeone.LiftConfig),
		CycleNumber: 1,
		RoutineIDs:  make(map[fivethreeone.Lift]map[int]string),
	}

	for _, lift := range fivethreeone.AllLifts() {
		templateID, err := fivethreeone.FindExerciseTemplateID(ctx, client, lift)
		if err != nil {
			slog.Error("finding exercise template", "lift", lift.DisplayName(), "error", err)
			os.Exit(1)
		}
		bbbTemplateID, err := fivethreeone.FindBBBExerciseTemplateID(ctx, client, lift)
		if err != nil {
			slog.Error("finding BBB exercise template", "lift", lift.DisplayName(), "error", err)
			os.Exit(1)
		}
		fmt.Printf("%s — found exercise templates\n", lift.DisplayName())

		fmt.Printf("training max for %s (kg): ", lift.DisplayName())
		scanner.Scan()
		var tm float64
		if _, err := fmt.Sscanf(scanner.Text(), "%f", &tm); err != nil {
			slog.Error("invalid training max", "lift", lift.DisplayName(), "error", err)
			os.Exit(1)
		}

		cfg.Lifts[lift] = fivethreeone.LiftConfig{
			TrainingMaxKg:         tm,
			ExerciseTemplateID:    templateID,
			BBBExerciseTemplateID: bbbTemplateID,
		}
	}

	if !*configOnly {
		folderID, err := create531Folder(ctx, client, cfg.CycleNumber)
		if err != nil {
			slog.Error("creating folder", "error", err)
			os.Exit(1)
		}
		cfg.FolderID = &folderID

		syncer := fivethreeone.NewSyncer(client, cfg)
		if err := syncer.SyncRoutines(ctx); err != nil {
			slog.Error("creating routines", "error", err)
			os.Exit(1)
		}
	}

	if err := fivethreeone.SaveConfig(*configPath, cfg); err != nil {
		slog.Error("saving config", "error", err)
		os.Exit(1)
	}

	if *configOnly {
		fmt.Printf("\nConfig-only init complete. Config saved to %s — run 'hevy 531 sync' to create routines.\n", *configPath)
	} else {
		fmt.Printf("\n5/3/1 program initialized! Config saved to %s\n", *configPath)
		fmt.Println("Run 'hevy 531 status --config=" + *configPath + "' to see your program.")
	}
}

func create531Folder(ctx context.Context, client *hevy.Client, cycleNumber int) (int, error) {
	title := fmt.Sprintf("5/3/1 (Cycle %d)", cycleNumber)
	folder, err := client.CreateRoutineFolder(ctx, &hevy.RoutineFolderRequest{Title: title})
	if err != nil {
		return 0, fmt.Errorf("creating folder: %w", err)
	}
	return folder.ID, nil
}

func cmd531Sync(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("531 sync", flag.ExitOnError)
	configPath := fs.String("config", "531.json", "path to 5/3/1 config file")
	nextCycle := fs.Bool("next-cycle", false, "advance to the next cycle: create a fresh routine folder and routines (training maxes are left untouched — use 'hevy 531 tm' to change them)")
	fs.Parse(args)

	cfg, err := fivethreeone.LoadConfig(*configPath)
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	if *nextCycle {
		cfg.CycleNumber++
		cfg.RoutineIDs = nil
		folderID, err := create531Folder(ctx, client, cfg.CycleNumber)
		if err != nil {
			slog.Error("creating folder", "error", err)
			os.Exit(1)
		}
		cfg.FolderID = &folderID
		fmt.Printf("Advancing to cycle %d — created folder \"5/3/1 (Cycle %d)\".\n", cfg.CycleNumber, cfg.CycleNumber)
	}

	syncer := fivethreeone.NewSyncer(client, cfg)
	if err := syncer.SyncRoutines(ctx); err != nil {
		slog.Error("syncing routines", "error", err)
		os.Exit(1)
	}

	if err := fivethreeone.SaveConfig(*configPath, cfg); err != nil {
		slog.Error("saving config", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Routines synced for Cycle %d\n", cfg.CycleNumber)
}

func cmd531FixExercises(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("531 fix-exercises", flag.ExitOnError)
	configPath := fs.String("config", "531.json", "path to 5/3/1 config file")
	fs.Parse(args)

	cfg, err := fivethreeone.LoadConfig(*configPath)
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	if err := fivethreeone.RefreshExerciseTemplateIDs(ctx, client, cfg); err != nil {
		slog.Error("refreshing exercise template IDs", "error", err)
		os.Exit(1)
	}

	if err := fivethreeone.SaveConfig(*configPath, cfg); err != nil {
		slog.Error("saving config", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Exercise template IDs refreshed and saved to %s\n", *configPath)
}

func cmd531Status(args []string) {
	fs := flag.NewFlagSet("531 status", flag.ExitOnError)
	configPath := fs.String("config", "531.json", "path to 5/3/1 config file")
	fs.Parse(args)

	cfg, err := fivethreeone.LoadConfig(*configPath)
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Cycle:      %d\nAssistance: %s\n\n", cfg.CycleNumber, cfg.AssistanceScheme().DisplayName())

	for _, lift := range fivethreeone.AllLifts() {
		lc, ok := cfg.Lifts[lift]
		if !ok {
			continue
		}
		assistance := cfg.AssistanceSchemeFor(lift)

		fmt.Printf("%-16s TM: %.1f kg", lift.DisplayName(), lc.TrainingMaxKg)
		if weeks, exists := cfg.RoutineIDs[lift]; exists {
			fmt.Printf("  (%d routines configured)", len(weeks))
		}
		fmt.Println()

		for week := 1; week <= fivethreeone.DeloadWeek; week++ {
			fmt.Printf("  %s:\n", fivethreeone.WeekName(week))
			sets := fivethreeone.CalculateRoutineSets(lc.TrainingMaxKg, week, lc.UseLbs)
			for _, s := range sets {
				amrap := ""
				if s.IsAMRAP {
					amrap = "+"
				}
				fmt.Printf("    [%s] %.1f kg x%d%s\n", s.Type, s.WeightKg, s.Reps, amrap)
			}
			// Assistance sets are identical to one another, so print them as a count.
			if as := fivethreeone.CalculateAssistanceSets(lift, assistance, lc.TrainingMaxKg, week, lc.UseLbs); len(as) > 0 {
				label := strings.ToUpper(string(assistance))
				if cue := assistance.Cue(); cue != "" {
					label += ", " + cue
				}
				fmt.Printf("    [%s] %.1f kg x%d  (%d sets, %s)\n",
					as[0].Type, as[0].WeightKg, as[0].Reps, len(as), label)
			}
		}
		fmt.Println()
	}
}

// cmd531Assistance switches the supplemental-volume preset, for the whole program or —
// with --lift — for a single lift, which is how one lift runs a variant (paused FSL, say)
// without the program committing to it. The change only reaches Hevy on the next
// 'hevy 531 sync'.
func cmd531Assistance(args []string) {
	fs := flag.NewFlagSet("531 assistance", flag.ExitOnError)
	configPath := fs.String("config", "531.json", "path to 5/3/1 config file")
	liftName := fs.String("lift", "", "apply to this lift alone instead of the whole program")
	clearOverride := fs.Bool("clear", false, "with --lift, drop that lift's override so it follows the program preset again")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: hevy 531 assistance [bbb|fsl|fsl-paused|none] [--lift=LIFT] [--clear] [--config=FILE]")
		fs.PrintDefaults()
	}

	// The preset is an optional positional argument that comes first; the stdlib flag
	// package stops parsing at the first positional, so pull it off before the flags.
	preset := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		preset, args = args[0], args[1:]
	}
	fs.Parse(args)

	// An empty scheme means no preset was named, i.e. show rather than switch.
	var scheme fivethreeone.AssistanceScheme
	if preset != "" {
		parsed, ok := fivethreeone.ParseAssistanceScheme(preset)
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown assistance preset: %q (want one of bbb, fsl, fsl-paused, none)\n", preset)
			os.Exit(1)
		}
		scheme = parsed
	}

	cfg, err := fivethreeone.LoadConfig(*configPath)
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	if *liftName != "" {
		cmd531AssistanceLift(cfg, *configPath, *liftName, scheme, *clearOverride)
		return
	}
	if *clearOverride {
		fmt.Fprintln(os.Stderr, "--clear only applies together with --lift")
		os.Exit(1)
	}

	// No preset named: report the current one rather than changing anything.
	if scheme == "" {
		fmt.Printf("Assistance: %s\n", cfg.AssistanceScheme().DisplayName())
		printAssistanceOverrides(cfg)
		return
	}

	old := cfg.AssistanceScheme()
	cfg.Assistance = scheme

	if err := fivethreeone.SaveConfig(*configPath, cfg); err != nil {
		slog.Error("saving config", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Assistance: %s → %s\n", old.DisplayName(), scheme.DisplayName())
	printAssistanceOverrides(cfg)
	fmt.Println("Run 'hevy 531 sync' to push the updated routines to Hevy.")
}

// cmd531AssistanceLift handles the --lift form: showing, setting, or clearing one lift's
// override of the program-wide preset.
func cmd531AssistanceLift(cfg *fivethreeone.Config, configPath, liftName string, scheme fivethreeone.AssistanceScheme, clearOverride bool) {
	lift, ok := fivethreeone.ParseLift(liftName)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown lift: %q\n", liftName)
		os.Exit(1)
	}
	liftCfg, ok := cfg.Lifts[lift]
	if !ok {
		fmt.Fprintf(os.Stderr, "lift %s is not in the config\n", lift.DisplayName())
		os.Exit(1)
	}

	old := cfg.AssistanceSchemeFor(lift)

	switch {
	case clearOverride:
		liftCfg.Assistance = ""
	case scheme == "":
		// Nothing to change: report what this lift is doing and why.
		source := "program preset"
		if liftCfg.Assistance != "" {
			source = "lift override"
		}
		fmt.Printf("%s assistance: %s (%s)\n", lift.DisplayName(), old.DisplayNameFor(lift), source)
		return
	default:
		liftCfg.Assistance = scheme
	}

	cfg.Lifts[lift] = liftCfg
	if err := fivethreeone.SaveConfig(configPath, cfg); err != nil {
		slog.Error("saving config", "error", err)
		os.Exit(1)
	}

	now := cfg.AssistanceSchemeFor(lift)
	fmt.Printf("%s assistance: %s → %s\n", lift.DisplayName(), old.DisplayNameFor(lift), now.DisplayNameFor(lift))
	if liftCfg.Assistance == "" {
		fmt.Printf("%s now follows the program preset.\n", lift.DisplayName())
	}
	fmt.Println("Run 'hevy 531 sync' to push the updated routines to Hevy.")
}

// printAssistanceOverrides lists the lifts running something other than the program-wide
// preset, so a program-level report never hides a per-lift override.
func printAssistanceOverrides(cfg *fivethreeone.Config) {
	for _, lift := range fivethreeone.AllLifts() {
		if liftCfg, ok := cfg.Lifts[lift]; ok && liftCfg.Assistance != "" {
			fmt.Printf("  %s overrides: %s\n", lift.DisplayName(), liftCfg.Assistance.DisplayNameFor(lift))
		}
	}
}

// cmd531TM sets or adjusts the training max of a single lift. This is the only
// command that mutates a training max; routine planning ('531 sync') never
// touches it.
func cmd531TM(args []string) {
	fs := flag.NewFlagSet("531 tm", flag.ExitOnError)
	configPath := fs.String("config", "531.json", "path to 5/3/1 config file")
	set := fs.Float64("set", -1, "set the training max to this many kg")
	by := fs.Float64("by", 0, "adjust the training max by this many kg (may be negative)")
	increment := fs.Bool("increment", false, "bump the training max by the standard 5/3/1 amount (upper +2.5kg, lower +5kg)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: hevy 531 tm <squat|bench_press|overhead_press|deadlift> (--set N | --by N | --increment) [--config=FILE]")
		fs.PrintDefaults()
	}

	// The lift is a required positional argument that comes first; the stdlib
	// flag package stops parsing at the first positional, so pull it off before
	// parsing the remaining flags.
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fs.Usage()
		os.Exit(1)
	}
	liftArg := args[0]
	fs.Parse(args[1:])

	lift, ok := fivethreeone.ParseLift(liftArg)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown lift: %q (want one of squat, bench_press, overhead_press, deadlift)\n", liftArg)
		os.Exit(1)
	}

	// Exactly one of --set, --by, or --increment must be provided.
	modes := 0
	if *set >= 0 {
		modes++
	}
	if *by != 0 {
		modes++
	}
	if *increment {
		modes++
	}
	if modes != 1 {
		fmt.Fprintln(os.Stderr, "specify exactly one of --set, --by, or --increment")
		fs.Usage()
		os.Exit(1)
	}

	cfg, err := fivethreeone.LoadConfig(*configPath)
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	lc, ok := cfg.Lifts[lift]
	if !ok {
		fmt.Fprintf(os.Stderr, "%s is not configured in %s — run 'hevy 531 init' first\n", lift.DisplayName(), *configPath)
		os.Exit(1)
	}

	old := lc.TrainingMaxKg
	switch {
	case *set >= 0:
		lc.TrainingMaxKg = *set
	case *increment:
		lc.TrainingMaxKg += lift.TMIncrementKg()
	default:
		lc.TrainingMaxKg += *by
	}

	if lc.TrainingMaxKg < 0 {
		fmt.Fprintf(os.Stderr, "resulting training max would be negative (%.1f kg)\n", lc.TrainingMaxKg)
		os.Exit(1)
	}

	cfg.Lifts[lift] = lc

	if err := fivethreeone.SaveConfig(*configPath, cfg); err != nil {
		slog.Error("saving config", "error", err)
		os.Exit(1)
	}

	fmt.Printf("%s — training max %.1f kg → %.1f kg\n", lift.DisplayName(), old, lc.TrainingMaxKg)
	fmt.Println("Run 'hevy 531 sync' to push the updated routines to Hevy.")
}

// cmdFixDurations clears the durations the Strong import stamped onto every set.
//
// It is a dry run by default. Hevy's update endpoint replaces a workout wholesale, so a mistake
// here is a mistake in the user's training history, and the scan is cheap enough to always run
// first.
func cmdFixDurations(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("fix-durations", flag.ExitOnError)
	apply := fs.Bool("apply", false, "write the repairs (default: report what would change and stop)")
	limit := fs.Int("limit", 0, "repair at most N workouts, oldest first (0 = no limit)")
	verbose := fs.Bool("v", false, "list every affected workout, not just the ones Hevy displays")
	// The API never returns is_private, but the update endpoint always sets it, so the value has to
	// be supplied rather than preserved. Defaulting to false matches a default Hevy account; anyone
	// whose history is private must say so or the repair would publish it.
	private := fs.Bool("private", false, "mark repaired workouts private (the API cannot read back the current setting)")
	fs.Parse(args)

	lookup, err := strongimport.LookupFromTemplates(ctx, client)
	if err != nil {
		slog.Error("loading exercise templates", "error", err)
		os.Exit(1)
	}

	report, err := strongimport.Scan(ctx, client, lookup)
	if err != nil {
		slog.Error("scanning workouts", "error", err)
		os.Exit(1)
	}

	findings := report.Findings
	sort.Slice(findings, func(i, j int) bool { return findings[i].Start.Before(findings[j].Start) })

	fmt.Printf("Scanned %d workouts.\n", report.Scanned)
	if len(findings) == 0 {
		fmt.Println("No stamped durations found.")
		return
	}

	fmt.Printf("Found %d workouts (%d sets) carrying the workout's own length as a set duration.\n",
		len(findings), report.Sets())
	fmt.Printf("%d of them show a bogus time in the Hevy UI:\n\n", report.Visible())

	for i := range findings {
		f := &findings[i]
		if len(f.VisibleExercises) == 0 && !*verbose {
			continue
		}
		fmt.Printf("  %s  %-28s %8s on %d sets", f.Start.Local().Format("2006-01-02 15:04"), f.Title,
			formatDuration(f.StampedSeconds), f.Sets)
		if len(f.VisibleExercises) > 0 {
			fmt.Printf("  <- shown on %s", strings.Join(f.VisibleExercises, ", "))
		}
		fmt.Println()
	}

	if *limit > 0 && *limit < len(findings) {
		findings = findings[:*limit]
	}

	if !*apply {
		fmt.Printf("\nDry run. Re-run with --apply to clear the duration on %d sets across %d workouts.\n",
			report.Sets(), len(report.Findings))
		return
	}

	fmt.Printf("\nRepairing %d workouts...\n", len(findings))

	var repaired int
	for i := range findings {
		f := &findings[i]

		// Re-read each workout immediately before writing it. The list response is a snapshot, and
		// the update endpoint replaces the whole workout, so writing a stale copy would discard any
		// edit made since the scan began.
		current, getErr := client.GetWorkout(ctx, f.WorkoutID)
		if getErr != nil {
			slog.Error("re-reading workout", "id", f.WorkoutID, "error", getErr)
			os.Exit(1)
		}

		if _, still := strongimport.Detect(current, lookup); !still {
			fmt.Printf("  skipped %s (%s): no longer damaged\n", f.Start.Format("2006-01-02"), f.Title)
			continue
		}

		current.IsPrivate = *private

		req, repairErr := strongimport.Repair(current)
		if repairErr != nil {
			slog.Error("building repair", "id", f.WorkoutID, "error", repairErr)
			os.Exit(1)
		}

		if _, updateErr := client.UpdateWorkout(ctx, f.WorkoutID, req); updateErr != nil {
			slog.Error("updating workout", "id", f.WorkoutID, "error", updateErr)
			os.Exit(1)
		}

		repaired++
		fmt.Printf("  [%d/%d] %s  %s\n", repaired, len(findings), f.Start.Format("2006-01-02"), f.Title)
	}

	fmt.Printf("Repaired %d workouts.\n", repaired)
}

// cmdRestoreDurations writes the per-set times from a Strong export back over the blanks the import
// left behind. It is the only path that recovers the real numbers rather than approximating them.
func cmdRestoreDurations(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("restore-durations", flag.ExitOnError)
	csvPath := fs.String("csv", "", "path to a Strong CSV export (required)")
	apply := fs.Bool("apply", false, "write the restorations (default: report what would change and stop)")
	private := fs.Bool("private", false, "mark rewritten workouts private (the API cannot read back the current setting)")
	fs.Parse(args)

	if *csvPath == "" {
		fmt.Fprintln(os.Stderr, "--csv is required, e.g. hevy restore-durations --csv strong.csv")
		os.Exit(1)
	}

	f, err := os.Open(*csvPath)
	if err != nil {
		slog.Error("opening Strong export", "path", *csvPath, "error", err)
		os.Exit(1)
	}
	defer f.Close()

	strong, err := strongimport.ParseStrong(f)
	if err != nil {
		slog.Error("parsing Strong export", "error", err)
		os.Exit(1)
	}

	workouts, err := hevy.Collect(client.ListWorkouts(ctx))
	if err != nil {
		slog.Error("listing workouts", "error", err)
		os.Exit(1)
	}

	plan, err := strongimport.BuildRestorePlan(strong, workouts)
	if err != nil {
		slog.Error("matching the export against the Hevy history", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Read %d timed workouts from %s; matched against %d Hevy workouts at UTC%+d.\n",
		len(strong), *csvPath, len(workouts), int(plan.Offset.Hours()))
	fmt.Printf("Recoverable: %d sets across %d exercises.\n\n", plan.Sets(), len(plan.Restorations))

	for i := range plan.Restorations {
		r := &plan.Restorations[i]
		fmt.Printf("  %s  %-30s %-18s %v\n", r.Start.Local().Format("2006-01-02"), r.Title, r.Exercise, r.Seconds)
	}

	if len(plan.Unmatched) > 0 {
		fmt.Printf("\nCould not resolve (left untouched):\n")
		for _, u := range plan.Unmatched {
			fmt.Printf("  %s\n", u)
		}
	}

	if !*apply {
		fmt.Printf("\nDry run. Re-run with --apply to write %d recovered times.\n", plan.Sets())
		return
	}

	byWorkout := map[string][]strongimport.Restoration{}
	var order []string
	for i := range plan.Restorations {
		id := plan.Restorations[i].WorkoutID
		if _, seen := byWorkout[id]; !seen {
			order = append(order, id)
		}
		byWorkout[id] = append(byWorkout[id], plan.Restorations[i])
	}

	fmt.Printf("\nRestoring %d workouts...\n", len(order))

	for n, id := range order {
		current, getErr := client.GetWorkout(ctx, id)
		if getErr != nil {
			slog.Error("re-reading workout", "id", id, "error", getErr)
			os.Exit(1)
		}
		current.IsPrivate = *private

		req, buildErr := strongimport.ApplyRestorations(current, byWorkout[id])
		if buildErr != nil {
			slog.Error("building restoration", "id", id, "error", buildErr)
			os.Exit(1)
		}

		if _, updateErr := client.UpdateWorkout(ctx, id, req); updateErr != nil {
			slog.Error("updating workout", "id", id, "error", updateErr)
			os.Exit(1)
		}

		fmt.Printf("  [%d/%d] %s  %s\n", n+1, len(order), current.StartTime.Local().Format("2006-01-02"), current.Title)
	}

	fmt.Printf("Restored %d sets across %d workouts.\n", plan.Sets(), len(order))
}

// fillFlag collects repeated --set NAME=SECONDS arguments.
type fillFlag map[string]int

func (f fillFlag) String() string { return "" }

func (f fillFlag) Set(v string) error {
	name, seconds, ok := strings.Cut(v, "=")
	if !ok {
		return fmt.Errorf("expected NAME=SECONDS, got %q", v)
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(seconds))
	if err != nil {
		return fmt.Errorf("parsing seconds in %q: %w", v, err)
	}
	if parsed <= 0 {
		return fmt.Errorf("seconds in %q must be positive", v)
	}

	f[strings.TrimSpace(name)] = parsed

	return nil
}

// cmdFillDurations writes an estimated duration into sets whose real value is unrecoverable.
//
// This is the last resort of the three duration commands, and the only one that writes a number
// nobody measured. It exists because Hevy renders a blank timed set as 0:00, which claims the set
// took no time at all; an estimate carrying a note saying it is an estimate is the closer of the
// two available lies to the truth.
func cmdFillDurations(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("fill-durations", flag.ExitOnError)
	fills := fillFlag{}
	fs.Var(fills, "set", "NAME=SECONDS to write into that exercise's blank sets (repeatable)")
	note := fs.String("note", strongimport.DefaultFillNote, "note recorded on every filled exercise; empty to skip")
	apply := fs.Bool("apply", false, "write the fills (default: report what would change and stop)")
	private := fs.Bool("private", false, "mark rewritten workouts private (the API cannot read back the current setting)")
	fs.Parse(args)

	lookup, err := strongimport.LookupFromTemplates(ctx, client)
	if err != nil {
		slog.Error("loading exercise templates", "error", err)
		os.Exit(1)
	}

	workouts, err := hevy.Collect(client.ListWorkouts(ctx))
	if err != nil {
		slog.Error("listing workouts", "error", err)
		os.Exit(1)
	}

	gaps := strongimport.FindGaps(workouts, lookup)
	if len(gaps) == 0 {
		fmt.Println("No blank timed sets found.")
		return
	}

	covered := map[string][]strongimport.Gap{}
	var skipped []strongimport.Gap
	for _, g := range gaps {
		if _, ok := fills[g.Exercise]; ok {
			covered[g.WorkoutID] = append(covered[g.WorkoutID], g)
		} else {
			skipped = append(skipped, g)
		}
	}

	var plannedSets int
	for _, gs := range covered {
		for _, g := range gs {
			plannedSets += g.Blank
		}
	}

	fmt.Printf("Found %d exercises with blank timed sets.\n\n", len(gaps))
	for _, g := range gaps {
		if seconds, ok := fills[g.Exercise]; ok {
			fmt.Printf("  %s  %-30s %-14s %d sets -> %ds each\n", g.LocalDate, g.Title, g.Exercise, g.Blank, seconds)
		}
	}

	if len(skipped) > 0 {
		fmt.Printf("\nNo --set value given, left blank:\n")
		for _, g := range skipped {
			fmt.Printf("  %s  %-30s %-14s %d sets\n", g.LocalDate, g.Title, g.Exercise, g.Blank)
		}
	}

	if plannedSets == 0 {
		fmt.Println("\nNothing to fill. Pass --set NAME=SECONDS for the exercises listed above.")
		return
	}

	if !*apply {
		fmt.Printf("\nDry run. Re-run with --apply to write %d estimated sets across %d workouts.\n",
			plannedSets, len(covered))
		return
	}

	ids := make([]string, 0, len(covered))
	for id := range covered {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return covered[ids[i]][0].LocalDate < covered[ids[j]][0].LocalDate })

	fmt.Printf("\nFilling %d workouts...\n", len(ids))

	var wrote int
	for n, id := range ids {
		current, getErr := client.GetWorkout(ctx, id)
		if getErr != nil {
			slog.Error("re-reading workout", "id", id, "error", getErr)
			os.Exit(1)
		}
		current.IsPrivate = *private

		req, filled, fillErr := strongimport.ApplyFills(current, fills, *note)
		if fillErr != nil {
			slog.Error("building fill", "id", id, "error", fillErr)
			os.Exit(1)
		}

		if _, updateErr := client.UpdateWorkout(ctx, id, req); updateErr != nil {
			slog.Error("updating workout", "id", id, "error", updateErr)
			os.Exit(1)
		}

		wrote += filled
		fmt.Printf("  [%d/%d] %s  %-30s %d sets\n", n+1, len(ids), covered[id][0].LocalDate, current.Title, filled)
	}

	fmt.Printf("Filled %d sets across %d workouts.\n", wrote, len(ids))
}
