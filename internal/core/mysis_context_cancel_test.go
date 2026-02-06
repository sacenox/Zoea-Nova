package core

import (
	"context"
	"testing"
	"time"
)

// TestContextCancellationPropagation verifies that when Stop() cancels the parent context,
// the child context in SendMessageFrom() receives the cancellation signal.
func TestContextCancellationPropagation(t *testing.T) {
	t.Run("parent cancel propagates to child", func(t *testing.T) {
		// Create parent context
		parent, parentCancel := context.WithCancel(context.Background())
		defer parentCancel()

		// Create child with timeout (same pattern as SendMessageFrom line 385)
		child, childCancel := context.WithTimeout(parent, 5*time.Second)
		defer childCancel()

		// Cancel parent (simulates Stop() line 284)
		parentCancel()

		// Verify child receives cancellation
		select {
		case <-child.Done():
			// SUCCESS: Child context canceled when parent canceled
			if err := child.Err(); err != context.Canceled {
				t.Errorf("expected context.Canceled, got %v", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("BUG: Child context not canceled after parent cancel")
		}
	})

	t.Run("child timeout does not affect parent", func(t *testing.T) {
		// Create parent context
		parent, parentCancel := context.WithCancel(context.Background())
		defer parentCancel()

		// Create child with very short timeout
		child, childCancel := context.WithTimeout(parent, 1*time.Millisecond)
		defer childCancel()

		// Wait for child timeout
		<-child.Done()
		if err := child.Err(); err != context.DeadlineExceeded {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}

		// Verify parent is still alive
		select {
		case <-parent.Done():
			t.Fatal("BUG: Parent context canceled when child timed out")
		case <-time.After(10 * time.Millisecond):
			// SUCCESS: Parent context still alive
		}
	})
}

// TestContextCancellationInProvider verifies that the mock provider respects context cancellation.
func TestContextCancellationInProvider(t *testing.T) {
	t.Run("provider respects canceled context", func(t *testing.T) {
		// Create mock provider with delay
		mock := NewMockProvider("test", "response").SetDelay(2 * time.Second)

		// Create context and cancel it
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Call provider - should fail quickly
		start := time.Now()
		_, err := mock.Chat(ctx, nil)
		elapsed := time.Since(start)

		// Verify it failed due to cancellation
		if err == nil {
			t.Fatal("expected error from canceled context")
		}
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}

		// Verify it didn't wait full delay
		if elapsed > 500*time.Millisecond {
			t.Errorf("provider took %v, expected quick failure on canceled context", elapsed)
		}
	})

	t.Run("provider respects context canceled during delay", func(t *testing.T) {
		// Create mock provider with delay
		mock := NewMockProvider("test", "response").SetDelay(2 * time.Second)

		// Create context
		ctx, cancel := context.WithCancel(context.Background())

		// Start provider call
		start := time.Now()
		errCh := make(chan error, 1)
		go func() {
			_, err := mock.Chat(ctx, nil)
			errCh <- err
		}()

		// Cancel after 100ms
		time.Sleep(100 * time.Millisecond)
		cancel()

		// Wait for result
		err := <-errCh
		elapsed := time.Since(start)

		// Verify it failed due to cancellation
		if err == nil {
			t.Fatal("expected error from canceled context")
		}
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}

		// Verify it didn't wait full delay
		if elapsed > 500*time.Millisecond {
			t.Errorf("provider took %v, expected quick failure after cancel", elapsed)
		}
	})
}

// Helper to create a mock provider for testing
func NewMockProvider(name, response string) *MockProvider {
	return &MockProvider{
		name:     name,
		response: response,
	}
}

// MockProvider is a minimal mock for testing context cancellation
type MockProvider struct {
	name     string
	response string
	delay    time.Duration
}

func (p *MockProvider) SetDelay(d time.Duration) *MockProvider {
	p.delay = d
	return p
}

func (p *MockProvider) Chat(ctx context.Context, messages interface{}) (string, error) {
	if p.delay > 0 {
		timer := time.NewTimer(p.delay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
			return p.response, nil
		}
	}
	return p.response, nil
}
