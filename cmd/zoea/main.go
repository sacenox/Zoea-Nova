package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
	"github.com/xonecas/zoea-nova/internal/tui"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	// Parse flags
	var (
		showVersion = flag.Bool("version", false, "Show version and exit")
		configPath  = flag.String("config", "config.toml", "Path to config file")
		debug       = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Zoea Nova %s\n", Version)
		os.Exit(0)
	}

	// Initialize logging
	if err := initLogging(*debug); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}

	log.Info().Str("version", Version).Msg("Starting Zoea Nova")

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}
	log.Debug().Interface("config", cfg).Msg("Configuration loaded")

	// Load credentials
	creds, err := config.LoadCredentials()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load credentials")
		creds = &config.Credentials{}
	}

	// Initialize store
	s, err := store.New()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize store")
	}
	defer s.Close()
	log.Debug().Msg("Store initialized")

	// Initialize event bus
	bus := core.NewEventBus(1000)
	defer bus.Close()

	// Initialize provider registry
	registry := initProviders(cfg, creds)
	log.Debug().Int("providers", len(registry.List())).Msg("Providers initialized")

	// Initialize commander
	commander := core.NewCommander(s, registry, bus, cfg)

	// Load existing agents
	if err := commander.LoadAgents(); err != nil {
		log.Warn().Err(err).Msg("Failed to load existing agents")
	}
	log.Debug().Int("agents", commander.AgentCount()).Msg("Agents loaded")

	// Initialize MCP proxy
	mcpProxy := mcp.NewProxy(cfg.MCP.Upstream)
	mcp.RegisterOrchestratorTools(mcpProxy, commander)
	log.Debug().Bool("upstream", mcpProxy.HasUpstream()).Int("local_tools", mcpProxy.LocalToolCount()).Msg("MCP proxy initialized")

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Subscribe to events for TUI
	eventCh := bus.Subscribe()

	// Create and run TUI
	model := tui.New(commander, s, eventCh)
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Handle shutdown in a goroutine
	go func() {
		<-sigCh
		log.Info().Msg("Received shutdown signal")
		commander.StopAll()
		program.Quit()
	}()

	// Run the TUI
	if _, err := program.Run(); err != nil {
		log.Fatal().Err(err).Msg("TUI error")
	}

	// Clean shutdown
	commander.StopAll()
	log.Info().Msg("Zoea Nova shutdown complete")
}

func initLogging(debug bool) error {
	// Ensure data directory exists
	dataDir, err := config.EnsureDataDir()
	if err != nil {
		return fmt.Errorf("ensure data dir: %w", err)
	}

	// Open log file (truncate on startup)
	logPath := filepath.Join(dataDir, "zoea.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Log to file only (TUI owns stdout/stderr)
	log.Logger = zerolog.New(logFile).With().Timestamp().Logger()

	return nil
}

func initProviders(cfg *config.Config, creds *config.Credentials) *provider.Registry {
	registry := provider.NewRegistry()

	// Register Ollama provider
	if ollCfg, ok := cfg.Providers["ollama"]; ok {
		p := provider.NewOllama(ollCfg.Endpoint, ollCfg.Model)
		registry.Register(p)
	}

	// Register OpenCode Zen provider
	if zenCfg, ok := cfg.Providers["opencode_zen"]; ok {
		apiKey := creds.GetAPIKey("opencode_zen")
		if apiKey != "" {
			p := provider.NewOpenCode(zenCfg.Endpoint, zenCfg.Model, apiKey)
			registry.Register(p)
		}
	}

	return registry
}
