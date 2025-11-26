package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Gappylul/goverseer"
)

// Permanent worker - always restarts
func permanentWorker(ctx context.Context) error {
	fmt.Println("Permanent worker running")
	time.Sleep(2 * time.Second)
	fmt.Println("Permanent worker exiting normally")
	return nil // Will restart even on normal exit
}

// Transient worker - restarts only on error
func transientWorker(ctx context.Context) error {
	fmt.Println("Transient worker running")
	time.Sleep(2 * time.Second)
	fmt.Println("Transient worker completed successfully")
	return nil // Will NOT restart (normal exit)
}

// Temporary worker - never restarts
func temporaryWorker(ctx context.Context) error {
	fmt.Println("Temporary worker running (initialization task)")
	time.Sleep(1 * time.Second)
	fmt.Println("Temporary worker done")
	return nil // Will NOT restart
}

func main() {
	sup := goverseer.New(
		goverseer.OneForOne,
		goverseer.WithName("restart-types-example"),
		goverseer.WithBackoff(goverseer.ConstantBackoff(500*time.Millisecond)),
		goverseer.WithEventHandler(func(e goverseer.Event) {
			switch e.Type {
			case goverseer.ChildStarted:
				log.Printf("✓ %s started", e.ChildName)
			case goverseer.ChildExited:
				log.Printf("✗ %s exited", e.ChildName)
			case goverseer.ChildRestarted:
				log.Printf("↻ %s restarted", e.ChildName)
			}
		}),
		goverseer.WithChildren(
			goverseer.ChildSpec{
				Name:    "permanent",
				Start:   permanentWorker,
				Restart: goverseer.Permanent,
			},
			goverseer.ChildSpec{
				Name:    "transient",
				Start:   transientWorker,
				Restart: goverseer.Transient,
			},
			goverseer.ChildSpec{
				Name:    "temporary",
				Start:   temporaryWorker,
				Restart: goverseer.Temporary,
			},
		),
	)

	if err := sup.Start(); err != nil {
		log.Fatal(err)
	}

	log.Println("Watch how different restart types behave:")
	log.Println("- Permanent: keeps restarting")
	log.Println("- Transient: exits after success")
	log.Println("- Temporary: exits and never restarts")
	log.Println()

	// Run for 10 seconds to observe behavior
	time.Sleep(10 * time.Second)
	sup.Stop()
}
