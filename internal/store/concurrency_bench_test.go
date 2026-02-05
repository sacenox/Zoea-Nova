package store

import (
	"math"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkConcurrentWrites(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")
	store, err := Open(dbPath)
	if err != nil {
		b.Fatalf("Open() error: %v", err)
	}
	defer store.Close()

	mysis, err := store.CreateMysis("bench", "ollama", "llama3", 0.7)
	if err != nil {
		b.Fatalf("CreateMysis() error: %v", err)
	}

	workers := 16
	durations := make([]time.Duration, b.N)
	var idx atomic.Int64
	var errCount atomic.Int64

	tasks := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range tasks {
				start := time.Now()
				err := store.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "bench", "", "")
				if err != nil {
					errCount.Add(1)
				}
				pos := idx.Add(1) - 1
				if pos >= 0 && int(pos) < len(durations) {
					durations[pos] = time.Since(start)
				}
			}
		}()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tasks <- struct{}{}
	}
	close(tasks)
	wg.Wait()
	b.StopTimer()

	if errCount.Load() > 0 {
		b.Fatalf("AddMemory errors: %d", errCount.Load())
	}

	count := int(idx.Load())
	if count == 0 {
		return
	}
	if count < len(durations) {
		durations = durations[:count]
	}

	samples := make([]int64, len(durations))
	for i, d := range durations {
		samples[i] = d.Nanoseconds()
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })

	median := samples[len(samples)/2]
	p99Index := int(math.Ceil(float64(len(samples))*0.99)) - 1
	if p99Index < 0 {
		p99Index = 0
	}
	if p99Index >= len(samples) {
		p99Index = len(samples) - 1
	}
	p99 := samples[p99Index]

	b.ReportMetric(float64(median)/1e6, "p50_ms")
	b.ReportMetric(float64(p99)/1e6, "p99_ms")
}
