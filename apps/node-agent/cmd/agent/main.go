package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Harsh223/PanelX/apps/node-agent/internal/config"
	"github.com/Harsh223/PanelX/apps/node-agent/internal/registration"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load node-agent config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	regClient := registration.NewClient(cfg)
	httpSrv := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           healthMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("node-agent http server failed: %v", err)
		}
	}()

	log.Printf("node-agent listening on %s", cfg.HTTPListenAddr)

	go heartbeatLoop(ctx, cfg, regClient)

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	log.Printf("node-agent stopped")
}

func healthMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func heartbeatLoop(ctx context.Context, cfg config.Config, regClient *registration.Client) {
	ticker := time.NewTicker(cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := regClient.Register(ctx, registration.RegistrationPayload{
				AgentID:   cfg.AgentID,
				Hostname:  "bootstrap-host",
				IPAddress: "127.0.0.1",
			})
			if err != nil {
				log.Printf("registration heartbeat failed: %v", err)
				continue
			}
			log.Printf("registration heartbeat sent")
		}
	}
}
