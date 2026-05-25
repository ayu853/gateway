package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ayu853/gateway/internal/analytics"
	"github.com/ayu853/gateway/internal/cache"
	"github.com/ayu853/gateway/internal/config"
	"github.com/ayu853/gateway/internal/healthcheck"
	"github.com/ayu853/gateway/internal/loadbalancer"
	"github.com/ayu853/gateway/internal/metrics"
	"github.com/ayu853/gateway/internal/middleware"
	"github.com/ayu853/gateway/internal/proxy"
)

const banner = `
██╗  ██╗██████╗      ██████╗  █████╗ ████████╗███████╗██╗    ██╗ █████╗ ██╗   ██╗
██║  ██║██╔══██╗    ██╔════╝ ██╔══██╗╚══██╔══╝██╔════╝██║    ██║██╔══██╗╚██╗ ██╔╝
███████║██████╔╝    ██║  ███╗███████║   ██║   █████╗  ██║ █╗ ██║███████║ ╚████╔╝
██╔══██║██╔═══╝     ██║   ██║██╔══██║   ██║   ██╔══╝  ██║███╗██║██╔══██║  ╚██╔╝
██║  ██║██║         ╚██████╔╝██║  ██║   ██║   ███████╗╚███╔███╔╝██║  ██║   ██║
╚═╝  ╚═╝╚═╝          ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚══════╝ ╚══╝╚══╝ ╚═╝  ╚═╝   ╚═╝
                    High-Performance API Gateway v1.0.0
`

func main() {
	configPath := flag.String("config", "config/gateway.yaml", "Path to configuration file")
	flag.Parse()

	fmt.Print(banner)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded from %s", *configPath)

	// Create backends
	backends := make([]*loadbalancer.Backend, len(cfg.Backends))
	for i, b := range cfg.Backends {
		backends[i] = loadbalancer.NewBackend(b.URL, b.Weight)
		log.Printf("  Backend: %s (weight: %d)", b.URL, b.Weight)
	}

	// Create load balancer
	balancer, err := loadbalancer.New(cfg.Balancer.Algorithm, backends)
	if err != nil {
		log.Fatalf("Failed to create load balancer: %v", err)
	}
	log.Printf("Load balancer: %s", cfg.Balancer.Algorithm)

	// Create components
	reverseProxy := proxy.New(balancer)
	analytics := analytics.New()
	appCache := cache.New(cfg.Cache.Enabled, cfg.Cache.TTL, cfg.Cache.MaxSize)
	appMetrics := metrics.New()
	rateLimiter := middleware.NewRateLimiter(cfg.Rate.Enabled, cfg.Rate.Requests, cfg.Rate.Burst)
	auth := middleware.NewAuth(cfg.Auth.Enabled, cfg.Auth.JWTSecret, cfg.Auth.APIKeys, cfg.Auth.ExcludePaths)
	circuitBreaker := middleware.NewCircuitBreaker(5, 3, 30*time.Second)

	// Start health checker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.Health.Enabled {
		healthChecker := healthcheck.New(
			backends,
			cfg.Health.Interval,
			cfg.Health.Timeout,
			cfg.Health.Path,
			cfg.Health.UnhealthyThreshold,
			cfg.Health.HealthyThreshold,
		)
		go healthChecker.Start(ctx)
	}

	// Build middleware chain (executed in reverse order)
	var handler http.Handler = reverseProxy
	handler = circuitBreaker.Middleware(handler)
	handler = auth.Middleware(handler)
	handler = rateLimiter.Middleware(handler)
	handler = analytics.Middleware(handler)
	handler = appMetrics.Middleware(handler)
	handler = middleware.CORS(middleware.DefaultCORSConfig())(handler)
	handler = middleware.Logger(handler)

	// Create router
	mux := http.NewServeMux()

	// Gateway admin endpoints
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		statuses := make([]map[string]interface{}, len(backends))
		for i, b := range backends {
			statuses[i] = map[string]interface{}{
				"url":                b.URL,
				"healthy":            b.IsHealthy(),
				"active_connections": b.GetActiveConnections(),
			}
		}
		data, _ := json.MarshalIndent(map[string]interface{}{
			"status":   "ok",
			"backends": statuses,
		}, "", "  ")
		w.Write(data)
	})

	mux.Handle("/api/analytics", analytics)

	mux.HandleFunc("/api/cache/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := json.MarshalIndent(appCache.Stats(), "", "  ")
		w.Write(data)
	})

	if cfg.Metrics.Enabled {
		mux.Handle(cfg.Metrics.Path, appMetrics)
		log.Printf("Metrics endpoint: %s", cfg.Metrics.Path)
	}

	// All other requests go through the proxy
	mux.Handle("/", handler)

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server
	go func() {
		log.Printf("")
		log.Printf("🚀 Gateway listening on http://localhost:%d", cfg.Server.Port)
		log.Printf("")
		log.Printf("  Admin Endpoints:")
		log.Printf("    GET /api/health     - Backend health status")
		log.Printf("    GET /api/analytics  - Request analytics")
		log.Printf("    GET /api/cache/stats - Cache statistics")
		if cfg.Metrics.Enabled {
			log.Printf("    GET %s       - Prometheus metrics", cfg.Metrics.Path)
		}
		log.Printf("")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("")
	log.Println("🛑 Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	cancel() // Stop health checks

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Gateway stopped.")
}
