package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Gappylul/goverseer"
)

func basicWorker(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	fmt.Println("Basic worker started")

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Basic worker shutting down")
			return nil
		case <-ticker.C:
			fmt.Println("Basic worker: tick")
		}
	}
}

func main() {
	sup := goverseer.New(
		goverseer.OneForOne,
		goverseer.WithName("basic-example"),
		goverseer.WithEventHandler(func(e goverseer.Event) {
			log.Printf("[%s] %s", e.Type, e.ChildName)
		}),
		goverseer.WithChildren(
			goverseer.ChildSpec{
				Name:    "worker-1",
				Start:   basicWorker,
				Restart: goverseer.Permanent,
			},
		),
	)

	if err := sup.Start(); err != nil {
		log.Fatal(err)
	}

	log.Println("Supervisor started. Press Ctrl+C to stop.")

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	if err := sup.Stop(); err != nil {
		log.Fatal(err)
	}

	log.Println("Shutdown complete")
}
