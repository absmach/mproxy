// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package breaker provides circuit breaker pattern for resilient backend calls.
package breaker

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrCircuitOpen is returned when the circuit breaker is open.
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half_open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

// Config holds circuit breaker configuration.
type Config struct {
	// MaxFailures is the number of failures before opening the circuit.
	MaxFailures int
	// ResetTimeout is how long to wait in Open state before transitioning to HalfOpen.
	ResetTimeout time.Duration
	// SuccessThreshold is the number of consecutive successes in HalfOpen before closing.
	SuccessThreshold int
	// Timeout is the maximum time allowed for a call.
	Timeout time.Duration
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu               sync.RWMutex
	config           Config
	state            State
	failures         int
	successes        int
	lastFailureTime  time.Time
	lastStateChange  time.Time
	onStateChange    func(from, to State)
}

// New creates a new circuit breaker.
func New(config Config) *CircuitBreaker {
	if config.MaxFailures == 0 {
		config.MaxFailures = 5
	}
	if config.ResetTimeout == 0 {
		config.ResetTimeout = 60 * time.Second
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 2
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &CircuitBreaker{
		config:          config,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// Call executes the given function if the circuit breaker allows it.
func (cb *CircuitBreaker) Call(fn func() error) error {
	if err := cb.beforeCall(); err != nil {
		return err
	}

	err := fn()

	cb.afterCall(err)
	return err
}

// beforeCall checks if the call is allowed.
func (cb *CircuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateOpen:
		// Check if we should transition to HalfOpen
		if time.Since(cb.lastStateChange) > cb.config.ResetTimeout {
			cb.setState(StateHalfOpen)
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		// Allow limited traffic in HalfOpen state
		return nil

	case StateClosed:
		return nil

	default:
		return ErrCircuitOpen
	}
}

// afterCall records the result of the call.
func (cb *CircuitBreaker) afterCall(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onFailure handles a failed call.
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.successes = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.config.MaxFailures {
			cb.setState(StateOpen)
		}

	case StateHalfOpen:
		// Any failure in HalfOpen immediately opens the circuit
		cb.setState(StateOpen)
	}
}

// onSuccess handles a successful call.
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		cb.failures = 0

	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.setState(StateClosed)
		}
	}
}

// setState changes the circuit breaker state.
func (cb *CircuitBreaker) setState(newState State) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	// Reset counters on state change
	if newState == StateClosed {
		cb.failures = 0
		cb.successes = 0
	} else if newState == StateHalfOpen {
		cb.successes = 0
	}

	// Notify state change
	if cb.onStateChange != nil {
		go cb.onStateChange(oldState, newState)
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// OnStateChange registers a callback for state changes.
func (cb *CircuitBreaker) OnStateChange(fn func(from, to State)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Stats returns circuit breaker statistics.
func (cb *CircuitBreaker) Stats() (state State, failures, successes int) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state, cb.failures, cb.successes
}
