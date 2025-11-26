package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/Gappylul/goverseer"
)

type Job struct {
	ID   int
	Data string
}

var jobQueue = make(chan Job, 100)

// Worker that processes jobs from a queue
func worker(id int) goverseer.ChildFunc {
	return func(ctx context.Context) error {
		fmt.Printf("Worker %d: Started\n", id)

		for {
			select {
			case <-ctx.Done():
				fmt.Printf("Worker %d: Shutting down\n", id)
				return nil

			case job := <-jobQueue:
				// Simulate work
				duration := time.Duration(rand.Intn(1000)) * time.Millisecond
				fmt.Printf("Worker %d: Processing job %d (takes %v)\n", id, job.ID, duration)
				time.Sleep(duration)
				fmt.Printf("Worker %d: Completed job %d\n", id, job.ID)
			}
		}
	}
}

// Job producer
func producer(ctx context.Context) error {
	jobID := 1
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			job := Job{ID: jobID, Data: fmt.Sprintf("data-%d", jobID)}
			jobQueue <- job
			fmt.Printf("Producer: Created job %d\n", jobID)
			jobID++
		}
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	sup := goverseer.New(
		goverseer.SimpleOneForOne,
		goverseer.WithName("worker-pool"),
		goverseer.WithEventHandler(func(e goverseer.Event) {
			if e.Type == goverseer.ChildRestarted {
				log.Printf("Worker crashed and restarted: %s", e.ChildName)
			}
		}),
	)

	if err := sup.Start(); err != nil {
		log.Fatal(err)
	}

	// Add producer
	sup.AddChild(goverseer.ChildSpec{
		Name:    "producer",
		Start:   producer,
		Restart: goverseer.Permanent,
	})

	// Add 5 workers dynamically
	for i := 1; i <= 5; i++ {
		sup.AddChild(goverseer.ChildSpec{
			Name:    fmt.Sprintf("worker-%d", i),
			Start:   worker(i),
			Restart: goverseer.Permanent,
		})
	}

	log.Println("Worker pool started with 5 workers")
	log.Println("Workers will process jobs from the queue")

	// Run for 20 seconds
	time.Sleep(20 * time.Second)

	// Scale up - add 3 more workers
	log.Println("\n🔼 Scaling up: Adding 3 more workers")
	for i := 6; i <= 8; i++ {
		sup.AddChild(goverseer.ChildSpec{
			Name:    fmt.Sprintf("worker-%d", i),
			Start:   worker(i),
			Restart: goverseer.Permanent,
		})
	}

	time.Sleep(10 * time.Second)

	// Scale down - remove some workers
	log.Println("\n🔽 Scaling down: Removing 4 workers")
	for i := 5; i <= 8; i++ {
		sup.RemoveChild(fmt.Sprintf("worker-%d", i))
	}

	time.Sleep(10 * time.Second)
	sup.Stop()
}
