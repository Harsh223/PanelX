package main

import (
	"log"

	"github.com/Harsh223/PanelX/apps/control-plane/internal/config"
	httpserver "github.com/Harsh223/PanelX/apps/control-plane/internal/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	srv := httpserver.New(cfg)
	log.Printf("panelx control-plane listening on %s:%d", cfg.HTTP.Host, cfg.HTTP.Port)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("control-plane server stopped: %v", err)
	}
}
