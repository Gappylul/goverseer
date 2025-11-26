package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Gappylul/goverseer"
)

// HTTP server worker
func httpServerWorker(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from supervised server!\n")
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK\n")
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		log.Println("HTTP Server: Listening on :8080")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP Server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("HTTP Server: Shutting down gracefully")
	return server.Shutdown(shutdownCtx)
}

// Request logger
func requestLogger(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var requestCount int

	for {
		select {
		case <-ctx.Done():
			log.Printf("Logger: Total requests served: %d", requestCount)
			return nil
		case <-ticker.C:
			requestCount += 100 // Simulated
			log.Printf("Logger: Requests in last 10s: ~100 (total: %d)", requestCount)
		}
	}
}

// Cleanup worker (runs once on startup)
func cleanupWorker(ctx context.Context) error {
	log.Println("Cleanup: Clearing old temp files...")
	time.Sleep(2 * time.Second)
	log.Println("Cleanup: Done")
	return nil // Will not restart (Temporary)
}

func main() {
	sup := goverseer.New(
		goverseer.OneForOne,
		goverseer.WithName("web-app"),
		goverseer.WithEventHandler(func(e goverseer.Event) {
			switch e.Type {
			case goverseer.ChildStarted:
				log.Printf("✓ %s started", e.ChildName)
			case goverseer.ChildExited:
				if e.Err != nil {
					log.Printf("✗ %s exited with error: %v", e.ChildName, e.Err)
				}
			case goverseer.ChildRestarted:
				log.Printf("↻ %s restarted", e.ChildName)
			}
		}),
		goverseer.WithChildren(
			goverseer.ChildSpec{
				Name:    "http-server",
				Start:   httpServerWorker,
				Restart: goverseer.Permanent,
			},
			goverseer.ChildSpec{
				Name:    "request-logger",
				Start:   requestLogger,
				Restart: goverseer.Permanent,
			},
			goverseer.ChildSpec{
				Name:    "cleanup",
				Start:   cleanupWorker,
				Restart: goverseer.Temporary,
			},
		),
	)

	if err := sup.Start(); err != nil {
		log.Fatal(err)
	}

	log.Println("Web application started!")
	log.Println("Try: curl http://localhost:8080")
	log.Println("     curl http://localhost:8080/health")
	log.Println("Press Ctrl+C to stop")

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("\nShutting down...")
	if err := sup.Stop(); err != nil {
		log.Fatal(err)
	}

	log.Println("Shutdown complete")
}
