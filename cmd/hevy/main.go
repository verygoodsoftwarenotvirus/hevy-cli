package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
	"github.com/verygoodsoftwarenotvirus/hevy-cli/fivethreeone"
)

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
  workouts count             Print total workout count
  workouts get <id>          Get a single workout
  routines list              List all routines
  routines get <id>          Get a single routine
  531 init --config=FILE     Set up 5/3/1 program
  531 sync --config=FILE     Update routines for current week
  531 advance --config=FILE  Advance to next week/cycle
  531 status --config=FILE   Print current program status

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
		fmt.Fprintln(os.Stderr, "Usage: hevy workouts <list|count|get> [args]")
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
		for _, e := range w.Exercises {
			fmt.Printf("\n  %s (%s)\n", e.Title, e.ExerciseTemplateID)
			if e.Notes != "" {
				fmt.Printf("    Notes: %s\n", e.Notes)
			}
			for _, s := range e.Sets {
				weight := ""
				if s.WeightKg != nil {
					weight = fmt.Sprintf("%.1f kg", *s.WeightKg)
				}
				reps := ""
				if s.Reps != nil {
					reps = fmt.Sprintf("x%d", *s.Reps)
				}
				fmt.Printf("    [%s] %s %s\n", s.Type, weight, reps)
			}
		}
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
		fmt.Printf("\n%s — %s\n", w.StartTime.Format("Mon 2006-01-02 15:04"), w.Title)
		for _, e := range w.Exercises {
			fmt.Printf("\n  %s\n", e.Title)
			if e.Notes != "" {
				fmt.Printf("    Notes: %s\n", e.Notes)
			}
			for _, s := range e.Sets {
				weight := ""
				if s.WeightKg != nil {
					weight = fmt.Sprintf("%.1f kg", *s.WeightKg)
				}
				reps := ""
				if s.Reps != nil {
					reps = fmt.Sprintf("x%d", *s.Reps)
				}
				rpe := ""
				if s.RPE != nil {
					rpe = fmt.Sprintf("  @RPE %.1f", *s.RPE)
				}
				fmt.Printf("    [%s] %s %s%s\n", s.Type, weight, reps, rpe)
			}
		}
	}
}

func cmdRoutines(ctx context.Context, client *hevy.Client, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hevy routines <list|get> [args]")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		for r, err := range client.ListRoutines(ctx) {
			if err != nil {
				slog.Error("listing routines", "error", err)
				os.Exit(1)
			}
			fmt.Printf("%s  %s  (%d exercises)\n", r.ID, r.Title, len(r.Exercises))
		}
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
		for _, e := range r.Exercises {
			fmt.Printf("\n  %s (%s)\n", e.Title, e.ExerciseTemplateID)
			for _, s := range e.Sets {
				weight := ""
				if s.WeightKg != nil {
					weight = fmt.Sprintf("%.1f kg", *s.WeightKg)
				}
				reps := ""
				if s.Reps != nil {
					reps = fmt.Sprintf("x%d", *s.Reps)
				}
				repRange := ""
				if s.RepRange != nil {
					repRange = fmt.Sprintf("x%d-%d", s.RepRange.Start, s.RepRange.End)
				}
				fmt.Printf("    [%s] %s %s%s\n", s.Type, weight, reps, repRange)
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown routines command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmd531(ctx context.Context, client *hevy.Client, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: hevy 531 <init|sync|advance|status> --config=FILE")
		os.Exit(1)
	}

	switch args[0] {
	case "init":
		cmd531Init(ctx, client, args[1:])
	case "sync":
		cmd531Sync(ctx, client, args[1:])
	case "advance":
		cmd531Advance(ctx, client, args[1:])
	case "status":
		cmd531Status(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown 531 command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmd531Init(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("531 init", flag.ExitOnError)
	configPath := fs.String("config", "531.json", "path to 5/3/1 config file")
	fs.Parse(args)

	scanner := bufio.NewScanner(os.Stdin)

	cfg := &fivethreeone.Config{
		Lifts:       make(map[fivethreeone.Lift]fivethreeone.LiftConfig),
		CycleNumber: 1,
		WeekNumber:  1,
		RoutineIDs:  make(map[fivethreeone.Lift]map[int]string),
	}

	for _, lift := range fivethreeone.AllLifts() {
		fmt.Printf("\n--- %s ---\n", lift.DisplayName())

		fmt.Printf("Exercise template ID (use 'hevy exercises search' to find): ")
		scanner.Scan()
		templateID := strings.TrimSpace(scanner.Text())
		if templateID == "" {
			slog.Error("exercise template ID is required", "lift", lift.DisplayName())
			os.Exit(1)
		}

		// Verify the template exists
		tmpl, err := client.GetExerciseTemplate(ctx, templateID)
		if err != nil {
			slog.Error("exercise template not found", "id", templateID, "error", err)
			os.Exit(1)
		}
		fmt.Printf("  Found: %s\n", tmpl.Title)

		fmt.Printf("Training max (kg): ")
		scanner.Scan()
		var tm float64
		if _, err := fmt.Sscanf(scanner.Text(), "%f", &tm); err != nil {
			slog.Error("invalid training max", "error", err)
			os.Exit(1)
		}

		cfg.Lifts[lift] = fivethreeone.LiftConfig{
			TrainingMaxKg:      tm,
			ExerciseTemplateID: templateID,
		}
	}

	// Create routines in Hevy
	syncer := fivethreeone.NewSyncer(client, cfg)
	if err := syncer.SyncRoutines(ctx); err != nil {
		slog.Error("creating routines", "error", err)
		os.Exit(1)
	}

	if err := fivethreeone.SaveConfig(*configPath, cfg); err != nil {
		slog.Error("saving config", "error", err)
		os.Exit(1)
	}

	fmt.Printf("\n5/3/1 program initialized! Config saved to %s\n", *configPath)
	fmt.Println("Run 'hevy 531 status --config=" + *configPath + "' to see your program.")
}

func cmd531Sync(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("531 sync", flag.ExitOnError)
	configPath := fs.String("config", "531.json", "path to 5/3/1 config file")
	fs.Parse(args)

	cfg, err := fivethreeone.LoadConfig(*configPath)
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
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

	fmt.Printf("Routines synced for Cycle %d, %s\n", cfg.CycleNumber, fivethreeone.WeekName(cfg.WeekNumber))
}

func cmd531Advance(ctx context.Context, client *hevy.Client, args []string) {
	fs := flag.NewFlagSet("531 advance", flag.ExitOnError)
	configPath := fs.String("config", "531.json", "path to 5/3/1 config file")
	fs.Parse(args)

	cfg, err := fivethreeone.LoadConfig(*configPath)
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	oldCycle := cfg.CycleNumber

	syncer := fivethreeone.NewSyncer(client, cfg)
	if err := syncer.AdvanceCycle(ctx); err != nil {
		slog.Error("advancing cycle", "error", err)
		os.Exit(1)
	}

	// Sync routines with updated training maxes
	if err := syncer.SyncRoutines(ctx); err != nil {
		slog.Error("syncing routines", "error", err)
		os.Exit(1)
	}

	if err := fivethreeone.SaveConfig(*configPath, cfg); err != nil {
		slog.Error("saving config", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Advanced from cycle %d to cycle %d\n", oldCycle, cfg.CycleNumber)
	fmt.Println("Updated training maxes:")
	for _, lift := range fivethreeone.AllLifts() {
		if lc, ok := cfg.Lifts[lift]; ok {
			fmt.Printf("  %s: %.1f kg\n", lift.DisplayName(), lc.TrainingMaxKg)
		}
	}
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

	fmt.Printf("Cycle:  %d\nWeek:   %d (%s)\n\n", cfg.CycleNumber, cfg.WeekNumber, fivethreeone.WeekName(cfg.WeekNumber))

	for _, lift := range fivethreeone.AllLifts() {
		lc, ok := cfg.Lifts[lift]
		if !ok {
			continue
		}
		fmt.Printf("%-16s TM: %.1f kg", lift.DisplayName(), lc.TrainingMaxKg)
		if weeks, exists := cfg.RoutineIDs[lift]; exists {
			fmt.Printf("  (%d routines configured)", len(weeks))
		}
		fmt.Println()

		sets := fivethreeone.CalculateRoutineSets(lc.TrainingMaxKg, cfg.WeekNumber, lc.UseLbs)
		for _, s := range sets {
			amrap := ""
			if s.IsAMRAP {
				amrap = "+"
			}
			fmt.Printf("  [%s] %.1f kg x%d%s\n", s.Type, s.WeightKg, s.Reps, amrap)
		}
		fmt.Println()
	}
}
