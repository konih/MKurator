package mqrest

import (
	"sync"
	"time"

	"github.com/conduit-ops/mkurator/internal/metrics"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

const (
	breakerStateClosed   = "closed"
	breakerStateOpen     = "open"
	breakerStateHalfOpen = "half_open"
)

type circuitBreakerConfig struct {
	failureThreshold int
	openTimeout      time.Duration
	now              func() time.Time
	onTransition     func(from, to string)
}

func defaultCircuitBreakerConfig() circuitBreakerConfig {
	return circuitBreakerConfig{
		failureThreshold: defaultFailureThreshold,
		openTimeout:      defaultOpenTimeout,
		now:              time.Now,
		onTransition:     metrics.RecordCircuitBreakerTransition,
	}
}

func circuitBreakerConfigFromResilience(cfg ResilienceConfig) circuitBreakerConfig {
	c := defaultCircuitBreakerConfig()
	if cfg.FailureThreshold > 0 {
		c.failureThreshold = cfg.FailureThreshold
	}
	if cfg.OpenTimeout > 0 {
		c.openTimeout = cfg.OpenTimeout
	}
	return c
}

type circuitBreaker struct {
	cfg              circuitBreakerConfig
	mu               sync.Mutex
	state            string
	consecutiveFails int
	openedAt         time.Time
	probeInFlight    bool
}

func newCircuitBreaker(cfg circuitBreakerConfig) *circuitBreaker {
	if cfg.failureThreshold <= 0 {
		cfg.failureThreshold = defaultFailureThreshold
	}
	if cfg.openTimeout <= 0 {
		cfg.openTimeout = defaultOpenTimeout
	}
	if cfg.now == nil {
		cfg.now = time.Now
	}
	return &circuitBreaker{cfg: cfg, state: breakerStateClosed}
}

func (b *circuitBreaker) beforeRequest() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case breakerStateClosed:
		return nil
	case breakerStateOpen:
		if b.cfg.now().Sub(b.openedAt) < b.cfg.openTimeout {
			return circuitOpenError()
		}
		b.transitionLocked(breakerStateHalfOpen)
		b.probeInFlight = true
		return nil
	case breakerStateHalfOpen:
		if b.probeInFlight {
			return circuitOpenError()
		}
		b.probeInFlight = true
		return nil
	default:
		return circuitOpenError()
	}
}

func (b *circuitBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutiveFails = 0
	b.probeInFlight = false
	if b.state != breakerStateClosed {
		b.transitionLocked(breakerStateClosed)
	}
}

func (b *circuitBreaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probeInFlight = false
	if b.state == breakerStateHalfOpen {
		b.transitionLocked(breakerStateOpen)
		b.openedAt = b.cfg.now()
		return
	}
	b.consecutiveFails++
	if b.consecutiveFails >= b.cfg.failureThreshold {
		b.transitionLocked(breakerStateOpen)
		b.openedAt = b.cfg.now()
	}
}

func (b *circuitBreaker) transitionLocked(to string) {
	if b.state == to {
		return
	}
	from := b.state
	b.state = to
	if b.cfg.onTransition != nil {
		b.cfg.onTransition(from, to)
	}
}

func circuitOpenError() error {
	return &mqadmin.TransientError{Message: "mqweb circuit breaker open"}
}
