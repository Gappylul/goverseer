package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Gappylul/goverseer"
)

// HTTP server worker
func httpServer(ctx context.Context) error {
	fmt.Println("HTTP Server: Starting on :8080")
	<-ctx.Done()
	fmt.Println("HTTP Server: Shutting down")
	return nil
}

// Health check worker
func healthCheck(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			fmt.Println("Health Check: System OK")
		}
	}
}

// Database connection pool
func dbPool(ctx context.Context) error {
	fmt.Println("DB Pool: Connected to database")
	<-ctx.Done()
	fmt.Println("DB Pool: Closing connections")
	return nil
}

// Cache worker
func cacheWorker(ctx context.Context) error {
	fmt.Println("Cache: Redis connected")
	<-ctx.Done()
	fmt.Println("Cache: Disconnecting")
	return nil
}

// Metrics collector
func metricsCollector(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			fmt.Println("Metrics: Collecting stats")
		}
	}
}

func main() {
	// Create HTTP supervisor
	httpSup := goverseer.New(
		goverseer.OneForAll, // If server fails, restart health check too
		goverseer.WithName("http-supervisor"),
		goverseer.WithChildren(
			goverseer.ChildSpec{
				Name:    "http-server",
				Start:   httpServer,
				Restart: goverseer.Permanent,
			},
			goverseer.ChildSpec{
				Name:    "health-check",
				Start:   healthCheck,
				Restart: goverseer.Permanent,
			},
		),
	)

	// Create database supervisor
	dbSup := goverseer.New(
		goverseer.OneForOne,
		goverseer.WithName("database-supervisor"),
		goverseer.WithChildren(
			goverseer.ChildSpec{
				Name:    "db-pool",
				Start:   dbPool,
				Restart: goverseer.Permanent,
			},
			goverseer.ChildSpec{
				Name:    "cache",
				Start:   cacheWorker,
				Restart: goverseer.Permanent,
			},
		),
	)

	// Create metrics supervisor
	metricsSup := goverseer.New(
		goverseer.OneForOne,
		goverseer.WithName("metrics-supervisor"),
		goverseer.WithChildren(
			goverseer.ChildSpec{
				Name:    "collector",
				Start:   metricsCollector,
				Restart: goverseer.Permanent,
			},
		),
	)

	// Create root supervisor managing all subsystems
	root := goverseer.New(
		goverseer.OneForOne, // Subsystems are independent
		goverseer.WithName("root-supervisor"),
		goverseer.WithEventHandler(func(e goverseer.Event) {
			log.Printf("[%s] %s: %s", e.Type, e.ChildName, e.Type.String())
		}),
	)

	// Start all subsystems
	if err := root.Start(); err != nil {
		log.Fatal(err)
	}

	// Add supervisors as children
	root.AddChild(goverseer.ChildSpec{
		Name:    "http-subsystem",
		Start:   func(ctx context.Context) error { httpSup.Start(); return httpSup.Wait() },
		Restart: goverseer.Permanent,
	})

	root.AddChild(goverseer.ChildSpec{
		Name:    "database-subsystem",
		Start:   func(ctx context.Context) error { dbSup.Start(); return dbSup.Wait() },
		Restart: goverseer.Permanent,
	})

	root.AddChild(goverseer.ChildSpec{
		Name:    "metrics-subsystem",
		Start:   func(ctx context.Context) error { metricsSup.Start(); return metricsSup.Wait() },
		Restart: goverseer.Permanent,
	})

	log.Println("Application started with supervision tree:")
	log.Println("Root")
	log.Println("├── HTTP (Server + HealthCheck)")
	log.Println("├── Database (Pool + Cache)")
	log.Println("└── Metrics (Collector)")
	log.Println()

	// Run for demo
	time.Sleep(30 * time.Second)
	root.Stop()
}
