//go:build ignore

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// TestBackend represents a simple test backend server.
type TestBackend struct {
	Name string
	Port int
}

func main() {
	// Support single-backend mode with -port flag
	port := flag.Int("port", 0, "Run a single backend on this port")
	name := flag.String("name", "", "Backend name")
	flag.Parse()

	if *port != 0 {
		// Single backend mode — runs as its own process
		n := *name
		if n == "" {
			n = fmt.Sprintf("backend-%d", *port)
		}
		log.Printf("✅ %s → http://localhost:%d", n, *port)
		startBackend(TestBackend{Name: n, Port: *port})
		return
	}

	// Multi-backend mode (default) — runs all 3 in one process
	backends := []TestBackend{
		{Name: "backend-alpha", Port: 9001},
		{Name: "backend-beta", Port: 9002},
		{Name: "backend-gamma", Port: 9003},
	}

	var wg sync.WaitGroup

	for _, b := range backends {
		wg.Add(1)
		go func(backend TestBackend) {
			defer wg.Done()
			startBackend(backend)
		}(b)
	}

	log.Println("========================================")
	log.Println("  Test Backends Running")
	log.Println("========================================")
	for _, b := range backends {
		log.Printf("  ✅ %s → http://localhost:%d", b.Name, b.Port)
	}
	log.Println("========================================")
	log.Println("  Press Ctrl+C to stop")
	log.Println("========================================")

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down test backends...")
}

func startBackend(b TestBackend) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Simulate variable response time (10-200ms)
		delay := time.Duration(10+rand.Intn(190)) * time.Millisecond
		time.Sleep(delay)

		w.Header().Set("Content-Type", "application/json")
		data, _ := json.MarshalIndent(map[string]interface{}{
			"server":    b.Name,
			"port":      b.Port,
			"path":      r.URL.Path,
			"method":    r.Method,
			"headers":   flattenHeaders(r.Header),
			"timestamp": time.Now().Format(time.RFC3339),
			"latency":   delay.String(),
		}, "", "  ")
		w.Write(data)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
			"server": b.Name,
		})
	})

	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"server":  b.Name,
			"message": "slow response",
		})
	})

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":  "simulated error",
			"server": b.Name,
		})
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", b.Port),
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Backend %s failed: %v", b.Name, err)
	}
}

func flattenHeaders(h http.Header) map[string]string {
	flat := make(map[string]string)
	for k, v := range h {
		if len(v) > 0 {
			flat[k] = v[0]
		}
	}
	return flat
}
