package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	hevy "github.com/verygoodsoftwarenotvirus/hevy-cli"
	"github.com/verygoodsoftwarenotvirus/hevy-cli/fivethreeone"
)

const defaultConfigPath = "531config.json"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: 531 <init|sync|advance> [--config path]")
		os.Exit(1)
	}

	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "path to config JSON file")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}

	apiKey := os.Getenv("HEVY_API_KEY")
	if apiKey == "" {
		log.Fatal("HEVY_API_KEY not set")
	}

	client := hevy.NewClient(apiKey)
	ctx := context.Background()

	switch cmd {
	case "init":
		cmdInit(ctx, client, *configPath)

	case "sync", "advance":
		cfg, err := fivethreeone.LoadConfig(*configPath)
		if err != nil {
			log.Fatalf("loading config: %v", err)
		}
		syncer := fivethreeone.NewSyncer(client, cfg)

		if cmd == "sync" {
			if err := syncer.SyncRoutines(ctx); err != nil {
				log.Fatalf("sync failed: %v", err)
			}
		} else {
			if err := syncer.AdvanceCycle(ctx); err != nil {
				log.Fatalf("advance failed: %v", err)
			}
		}

		if err := fivethreeone.SaveConfig(*configPath, cfg); err != nil {
			log.Fatalf("saving config: %v", err)
		}
		fmt.Println("Config saved.")

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q — use init, sync, or advance\n", cmd)
		os.Exit(1)
	}
}

func cmdInit(ctx context.Context, client *hevy.Client, configPath string) {
	scanner := bufio.NewScanner(os.Stdin)

	cfg := &fivethreeone.Config{
		Lifts:       make(map[fivethreeone.Lift]fivethreeone.LiftConfig),
		CycleNumber: 1,
		RoutineIDs:  make(map[fivethreeone.Lift]map[int]string),
	}

	for _, lift := range fivethreeone.AllLifts() {
		templateID, err := fivethreeone.FindExerciseTemplateID(ctx, client, lift)
		if err != nil {
			log.Fatalf("finding exercise for %s: %v", lift.DisplayName(), err)
		}
		bbbTemplateID, err := fivethreeone.FindBBBExerciseTemplateID(ctx, client, lift)
		if err != nil {
			log.Fatalf("finding BBB exercise for %s: %v", lift.DisplayName(), err)
		}
		fmt.Printf("%s — found exercise templates\n", lift.DisplayName())

		fmt.Printf("Training max for %s (kg): ", lift.DisplayName())
		scanner.Scan()
		var tm float64
		if _, err := fmt.Sscanf(scanner.Text(), "%f", &tm); err != nil {
			log.Fatalf("invalid training max for %s: %v", lift.DisplayName(), err)
		}

		cfg.Lifts[lift] = fivethreeone.LiftConfig{
			TrainingMaxKg:         tm,
			ExerciseTemplateID:    templateID,
			BBBExerciseTemplateID: bbbTemplateID,
		}
	}

	folderID, err := resolveFolder(ctx, client, scanner)
	if err != nil {
		log.Fatalf("resolving folder: %v", err)
	}
	cfg.FolderID = &folderID

	syncer := fivethreeone.NewSyncer(client, cfg)
	if err := syncer.SyncRoutines(ctx); err != nil {
		log.Fatalf("creating routines: %v", err)
	}

	if err := fivethreeone.SaveConfig(configPath, cfg); err != nil {
		log.Fatalf("saving config: %v", err)
	}

	fmt.Printf("\nInitialized! Config saved to %s\n", configPath)
}

const folderName = "Auto 5/3/1"

// resolveFolder finds or creates the "Auto 5/3/1" routine folder. If an existing one is
// found, the user is prompted to empty it before proceeding.
func resolveFolder(ctx context.Context, client *hevy.Client, scanner *bufio.Scanner) (int, error) {
	for folder, err := range client.ListRoutineFolders(ctx) {
		if err != nil {
			return 0, fmt.Errorf("listing folders: %w", err)
		}
		if folder.Title != folderName {
			continue
		}
		fmt.Printf("Folder %q already exists. Reuse it? [y/N]: ", folderName)
		scanner.Scan()
		if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			log.Fatal("aborted")
		}
		fmt.Println("Note: existing routines in the folder will remain — Hevy does not support deletion via API.")
		return folder.ID, nil
	}

	folder, err := client.CreateRoutineFolder(ctx, &hevy.RoutineFolderRequest{Title: folderName})
	if err != nil {
		return 0, fmt.Errorf("creating folder: %w", err)
	}
	return folder.ID, nil
}
