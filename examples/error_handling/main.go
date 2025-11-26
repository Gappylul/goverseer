package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/Gappylul/goverseer"
)

var attempt = 0

// Flaky worker that fails sometimes
func flakyWorker(ctx context.Context) error {
	attempt++
	fmt.Printf("Attempt #%d: Connecting to database...\n", attempt)

	// Simulate random failures
	if rand.Float32() < 0.6 { // 60% failure rate
		return errors.New("connection failed")
	}

	fmt.Println("✓ Connected successfully!")

	// Once connected, run until stopped
	<-ctx.Done()
	fmt.Println("Disconnecting...")
	return nil
}

// Panicking worker to demonstrate panic recovery
func panickyWorker(ctx context.Context) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic in worker (this shouldn't happen)")
		}
	}()

	time.Sleep(2 * time.Second)
	fmt.Println("Worker is about to panic!")
	panic("intentional panic for demonstration")
}

func main() {
	rand.Seed(time.Now().UnixNano())

	sup := goverseer.New(
		goverseer.OneForOne,
		goverseer.WithName("error-handling-example"),
		goverseer.WithIntensity(10, time.Minute),
		goverseer.WithBackoff(goverseer.ExponentialBackoff(
			100*time.Millisecond,
			5*time.Second,
		)),
		goverseer.WithEventHandler(func(e goverseer.Event) {
			switch e.Type {
			case goverseer.ChildExited:
				if e.Err != nil {
					log.Printf("⚠ %s failed: %v", e.ChildName, e.Err)
				}
			case goverseer.ChildRestarted:
				log.Printf("↻ %s restarting...", e.ChildName)
			case goverseer.ChildPanicked:
				log.Printf("💥 %s panicked: %v", e.ChildName, e.Err)
			}
		}),
		goverseer.WithChildren(
			goverseer.ChildSpec{
				Name:    "flaky-db",
				Start:   flakyWorker,
				Restart: goverseer.Permanent,
			},
		),
	)

	if err := sup.Start(); err != nil {
		log.Fatal(err)
	}

	log.Println("Demonstrating automatic retry with backoff...")
	time.Sleep(20 * time.Second)
	sup.Stop()
}
