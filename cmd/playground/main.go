package main

import (
	"context"
	"fmt"
	"log"
	"os"

	hevy "github.com/verygoodsoftwarenotvirus/hevy-cli"
)

func main() {
	apiKey := os.Getenv("HEVY_API_KEY")
	if apiKey == "" {
		log.Fatal("HEVY_API_KEY not set")
	}

	client := hevy.NewClient(apiKey)
	ctx := context.Background()

	for routine, err := range client.ListRoutines(ctx) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s\t%s\n", routine.ID, routine.Title)
	}
}
