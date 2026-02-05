package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestRenderMysisLineWithAccount(t *testing.T) {
	// Force color output for testing
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	info := MysisInfo{
		ID:              "abc123",
		Name:            "test-mysis",
		State:           "running",
		Provider:        "ollama",
		AccountUsername: "crab_miner",
		LastMessage:     "Mining asteroid",
		CreatedAt:       time.Now(),
	}

	width := 100
	line := renderMysisLine(info, false, false, "⬡", width)

	// Should contain account username
	if !strings.Contains(line, "crab_miner") {
		t.Error("Expected account username 'crab_miner' in mysis line")
	}

	// Verify ANSI codes are present (styling is applied)
	if !strings.Contains(line, "\x1b[") {
		t.Error("Expected ANSI escape codes (styling) in output")
	}
}

func TestRenderMysisLineWithoutAccount(t *testing.T) {
	// Force color output
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	info := MysisInfo{
		ID:              "abc123",
		Name:            "test-mysis",
		State:           "idle",
		Provider:        "ollama",
		AccountUsername: "", // No account claimed yet
		LastMessage:     "",
		CreatedAt:       time.Now(),
	}

	width := 100
	line := renderMysisLine(info, false, false, "⬡", width)

	// Should contain indicator for no account
	if !strings.Contains(line, "no account") {
		t.Error("Expected 'no account' indicator when AccountUsername is empty")
	}
}
