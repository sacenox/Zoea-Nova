package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

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

// logFile is the global log file handle, closed on shutdown.
var logFile *os.File

func main() {
	// Parse flags
	var (
		showVersion = flag.Bool("version", false, "Show version and exit")
		configPath  = flag.String("config", "config.toml", "Path to config file")
		debug       = flag.Bool("debug", false, "Enable debug logging")
		testMCP     = flag.Bool("test-mcp", false, "Test MCP connection and tool calling, then exit")
		offline     = flag.Bool("offline", false, "Run in offline mode with stub MCP server")
		startSwarm  = flag.Bool("start-swarm", false, "Auto-start all idle myses on launch")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Zoea Nova %s\n", Version)
		os.Exit(0)
	}

	if *testMCP {
		runMCPTest(*configPath)
		return
	}

	// Initialize logging
	if err := initLogging(*debug); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

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

	// Release all accounts on startup to clear stale in_use flags from crashes
	if err := s.ReleaseAllAccounts(); err != nil {
		log.Warn().Err(err).Msg("Failed to release accounts on startup")
	} else {
		log.Info().Msg("Released all accounts on startup")
	}

	// Initialize event bus
	bus := core.NewEventBus(1000)
	// Note: bus.Close() is called explicitly in onQuit callback and signal handler
	// No defer needed here to avoid duplicate closes

	// Initialize provider registry
	registry := initProviders(cfg, creds)
	log.Debug().Int("providers", len(registry.List())).Msg("Providers initialized")

	// Determine MCP endpoint for myses
	var mcpEndpoint string
	if *offline {
		log.Info().Msg("Running in offline mode - myses will use stub MCP client")
		// Empty endpoint signals offline mode
	} else if cfg.MCP.Upstream != "" {
		mcpEndpoint = cfg.MCP.Upstream
		log.Info().Str("endpoint", mcpEndpoint).Msg("MCP endpoint configured - each mysis will create its own client")
	}

	// Initialize commander with MCP endpoint
	commander := core.NewCommander(s, registry, bus, cfg, mcpEndpoint)

	// Load existing myses from database
	if err := commander.LoadMyses(); err != nil {
		log.Warn().Err(err).Msg("Failed to load existing myses")
	}
	log.Debug().Int("myses", commander.MysisCount()).Msg("Myses loaded")

	// Auto-start all existing myses on launch
	// Each mysis will create its own MCP client during Start()
	for _, a := range commander.ListMyses() {
		if err := a.Start(); err != nil {
			log.Warn().Err(err).Str("mysis", a.Name()).Msg("Failed to start mysis")
		}
	}

	// Log goroutine count at startup for leak detection
	log.Info().Int("goroutines", runtime.NumGoroutine()).Msg("Application started")

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Subscribe to events for TUI
	eventCh := bus.Subscribe()

	// Create and run TUI
	model := tui.New(commander, s, eventCh, *startSwarm, cfg)

	// Set cleanup callback to close event bus before quit
	model.SetOnQuit(func() {
		bus.Close()
	})

	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Handle shutdown in a goroutine
	go func() {
		<-sigCh
		log.Info().Msg("Received shutdown signal")
		// Don't call StopAll here - let main cleanup handle it after program.Run()
		// This avoids stopping myses twice
		bus.Close() // Close event bus to unblock TUI event listener
		program.Quit()
	}()

	// Run the TUI
	if _, err := program.Run(); err != nil {
		log.Fatal().Err(err).Msg("TUI error")
	}

	// Clean shutdown
	log.Info().Int("goroutines", runtime.NumGoroutine()).Msg("Shutdown initiated")
	commander.StopAll()

	// Close bus (idempotent if already closed by onQuit or signal handler)
	bus.Close()

	// Release all accounts
	if err := s.ReleaseAllAccounts(); err != nil {
		log.Warn().Err(err).Msg("Failed to release accounts on shutdown")
	}

	log.Info().Int("goroutines", runtime.NumGoroutine()).Msg("Zoea Nova shutdown complete")
}

func initLogging(debug bool) error {
	// Ensure data directory exists
	dataDir, err := config.EnsureDataDir()
	if err != nil {
		return fmt.Errorf("ensure data dir: %w", err)
	}

	// Open log file (truncate on startup)
	logPath := filepath.Join(dataDir, "zoea.log")
	var openErr error
	logFile, openErr = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if openErr != nil {
		return fmt.Errorf("open log file: %w", openErr)
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

	for name, provCfg := range cfg.Providers {
		// Detect provider type by endpoint
		if strings.Contains(provCfg.Endpoint, "localhost:11434") || strings.Contains(provCfg.Endpoint, "/ollama") {
			// Ollama-based provider
			factory := provider.NewOllamaFactory(name, provCfg.Endpoint)
			registry.RegisterFactory(name, factory)
		} else if strings.Contains(provCfg.Endpoint, "opencode.ai") {
			// OpenCode-based provider
			// Use explicit api_key_name if provided, otherwise use provider config name
			keyName := provCfg.APIKeyName
			if keyName == "" {
				keyName = name
			}
			apiKey := creds.GetAPIKey(keyName)
			if apiKey != "" {
				factory := provider.NewOpenCodeFactory(name, provCfg.Endpoint, apiKey)
				registry.RegisterFactory(name, factory)
			}
		}
	}

	return registry
}

type accountStoreAdapter struct {
	store *store.Store
}

func (a *accountStoreAdapter) CreateAccount(username, password string, mysisID ...string) (*mcp.Account, error) {
	acc, err := a.store.CreateAccount(username, password, mysisID...)
	if err != nil {
		return nil, err
	}
	return &mcp.Account{Username: acc.Username, Password: acc.Password}, nil
}

func (a *accountStoreAdapter) MarkAccountInUse(username, mysisID string) error {
	return a.store.MarkAccountInUse(username, mysisID)
}

func (a *accountStoreAdapter) ReleaseAccount(username string) error {
	return a.store.ReleaseAccount(username)
}

func (a *accountStoreAdapter) ReleaseAllAccounts() error {
	return a.store.ReleaseAllAccounts()
}

func (a *accountStoreAdapter) ClaimAccount(mysisID string) (*mcp.Account, error) {
	acc, err := a.store.ClaimAccount(mysisID)
	if err != nil {
		return nil, err
	}
	return &mcp.Account{Username: acc.Username, Password: acc.Password}, nil
}

type commanderAdapter struct {
	commander *core.Commander
}

func (a *commanderAdapter) MysisCount() int {
	return a.commander.MysisCount()
}

func (a *commanderAdapter) MaxMyses() int {
	return a.commander.MaxMyses()
}

func (a *commanderAdapter) GetStateCounts() map[string]int {
	return a.commander.GetStateCounts()
}

func (a *commanderAdapter) SendMessageAsync(mysisID, message string) error {
	return a.commander.SendMessageAsync(mysisID, message)
}

func (a *commanderAdapter) BroadcastAsync(message string) error {
	return a.commander.BroadcastAsync(message)
}

func (a *commanderAdapter) BroadcastFrom(senderID, message string) error {
	return a.commander.BroadcastFrom(senderID, message)
}

func (a *commanderAdapter) SearchMessages(mysisID, query string, limit int) ([]mcp.SearchResult, error) {
	memories, err := a.commander.Store().SearchMemories(mysisID, query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]mcp.SearchResult, len(memories))
	for i, m := range memories {
		results[i] = mcp.SearchResult{
			Role:      string(m.Role),
			Source:    string(m.Source),
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	return results, nil
}

func (a *commanderAdapter) SearchReasoning(mysisID, query string, limit int) ([]mcp.ReasoningResult, error) {
	memories, err := a.commander.Store().SearchReasoning(mysisID, query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]mcp.ReasoningResult, len(memories))
	for i, m := range memories {
		results[i] = mcp.ReasoningResult{
			Role:      string(m.Role),
			Source:    string(m.Source),
			Content:   m.Content,
			Reasoning: m.Reasoning,
			CreatedAt: m.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	return results, nil
}

// runMCPTest tests the MCP connection and tool calling.
func runMCPTest(configPath string) {
	fmt.Println("=== MCP Tool Test ===")
	fmt.Println()

	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("ERROR: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create MCP proxy
	fmt.Printf("Upstream MCP: %s\n", cfg.MCP.Upstream)
	var upstreamClient mcp.UpstreamClient
	if cfg.MCP.Upstream != "" {
		upstreamClient = mcp.NewClient(cfg.MCP.Upstream)
	}
	mcpProxy := mcp.NewProxy(upstreamClient)

	// Create a mock orchestrator for local tools
	mockOrch := &mockOrchestrator{}
	mcp.RegisterOrchestratorTools(mcpProxy, mockOrch)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize upstream if configured
	if mcpProxy.HasUpstream() {
		fmt.Println("\nInitializing upstream MCP connection...")
		if err := mcpProxy.Initialize(ctx); err != nil {
			fmt.Printf("ERROR: Failed to initialize upstream: %v\n", err)
			fmt.Println("(Continuing with local tools only)")
		} else {
			fmt.Println("OK: Upstream initialized")
		}
	} else {
		fmt.Println("\nNo upstream configured - local tools only")
	}

	// List all tools
	fmt.Println("\n--- Available Tools ---")
	tools, err := mcpProxy.ListTools(ctx)
	if err != nil {
		fmt.Printf("ERROR: Failed to list tools: %v\n", err)
		fmt.Println("(This may indicate the upstream server doesn't support tools/list or returned an error)")
	}

	if len(tools) == 0 {
		fmt.Println("WARNING: No tools available!")
	} else {
		localCount := 0
		upstreamCount := 0
		for _, t := range tools {
			prefix := "  "
			if len(t.Name) > 5 && t.Name[:5] == "zoea_" {
				prefix = "  [local] "
				localCount++
			} else {
				prefix = "  [upstream] "
				upstreamCount++
			}
			fmt.Printf("%s%s - %s\n", prefix, t.Name, t.Description)
		}
		fmt.Printf("\nTotal: %d tools (%d local, %d upstream)\n", len(tools), localCount, upstreamCount)
	}

	// Test calling a local tool - removed zoea_swarm_status as it has been deleted

	// Test calling an upstream tool if available
	if mcpProxy.HasUpstream() {
		fmt.Println("\n--- Testing Upstream Tool Call ---")
		// Try to find a non-zoea tool to call
		var upstreamTool *mcp.Tool
		for i := range tools {
			if len(tools[i].Name) < 5 || tools[i].Name[:5] != "zoea_" {
				upstreamTool = &tools[i]
				break
			}
		}
		if upstreamTool != nil {
			fmt.Printf("Calling: %s\n", upstreamTool.Name)
			result, err := mcpProxy.CallTool(ctx, mcp.CallerContext{}, upstreamTool.Name, nil)
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
			} else if result.IsError {
				fmt.Printf("TOOL ERROR: %s\n", result.Content[0].Text)
			} else {
				text := result.Content[0].Text
				if len(text) > 200 {
					text = text[:197] + "..."
				}
				fmt.Printf("OK: %s\n", text)
			}
		} else {
			fmt.Println("No upstream tools found to test")
		}
	}

	fmt.Println("\n=== Test Complete ===")
}

// mockOrchestrator is a simple orchestrator for testing.
type mockOrchestrator struct{}

func (m *mockOrchestrator) MysisCount() int {
	return 0
}

func (m *mockOrchestrator) MaxMyses() int {
	return 16
}

func (m *mockOrchestrator) GetStateCounts() map[string]int {
	return map[string]int{
		"running": 0,
		"idle":    0,
		"stopped": 0,
		"errored": 0,
	}
}

func (m *mockOrchestrator) SendMessageAsync(mysisID, message string) error {
	return fmt.Errorf("not available in test mode")
}

func (m *mockOrchestrator) BroadcastAsync(message string) error {
	return fmt.Errorf("not available in test mode")
}

func (m *mockOrchestrator) BroadcastFrom(senderID, message string) error {
	return fmt.Errorf("not available in test mode")
}

func (m *mockOrchestrator) SearchMessages(mysisID, query string, limit int) ([]mcp.SearchResult, error) {
	return []mcp.SearchResult{}, nil
}

func (m *mockOrchestrator) SearchReasoning(mysisID, query string, limit int) ([]mcp.ReasoningResult, error) {
	return []mcp.ReasoningResult{}, nil
}
