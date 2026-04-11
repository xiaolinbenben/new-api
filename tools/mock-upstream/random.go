package main

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type lockedRand struct {
	mu  sync.Mutex
	rnd *rand.Rand
}

func newLockedRand(cfg mockConfig) *lockedRand {
	seed := time.Now().UnixNano()
	if cfg.Random.Mode == "seeded" {
		seed = cfg.Random.Seed
	}
	return &lockedRand{
		rnd: rand.New(rand.NewSource(seed)),
	}
}

func (r *lockedRand) intn(max int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if max <= 0 {
		return 0
	}
	return r.rnd.Intn(max)
}

func (r *lockedRand) intRange(v intRange) int {
	if v.Max <= v.Min {
		return v.Min
	}
	return v.Min + r.intn(v.Max-v.Min+1)
}

func (r *lockedRand) floatRange(v floatRange) float64 {
	if v.Max <= v.Min {
		return v.Min
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return v.Min + r.rnd.Float64()*(v.Max-v.Min)
}

func (r *lockedRand) bool(probability float64) bool {
	if probability <= 0 {
		return false
	}
	if probability >= 1 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rnd.Float64() < probability
}

func (r *lockedRand) pick(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	index := r.intn(len(values))
	return values[index]
}

func (r *lockedRand) shuffleStrings(values []string) []string {
	out := append([]string(nil), values...)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rnd.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

func (r *lockedRand) fillBytes(buf []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = r.rnd.Read(buf)
}

func (r *lockedRand) randomID(prefix string) string {
	return fmt.Sprintf("%s_%08x", prefix, r.intn(1<<30))
}

func (r *lockedRand) randomText(tokens int) string {
	if tokens <= 0 {
		return ""
	}
	words := []string{
		"mock", "new", "api", "stream", "token", "latency", "vector", "image", "video", "assistant",
		"upstream", "simulated", "payload", "random", "system", "response", "tool", "context", "model", "reasoning",
		"gateway", "worker", "control", "pressure", "cluster", "session", "trace", "result", "object", "message",
	}
	var builder strings.Builder
	for i := 0; i < tokens; i++ {
		if i > 0 {
			if i%17 == 0 {
				builder.WriteString(". ")
			} else {
				builder.WriteByte(' ')
			}
		}
		word := words[r.intn(len(words))]
		builder.WriteString(word)
		if i%31 == 0 && i > 0 {
			builder.WriteString(fmt.Sprintf("-%d", 100+r.intn(900)))
		}
	}
	if !strings.HasSuffix(builder.String(), ".") {
		builder.WriteByte('.')
	}
	return builder.String()
}
