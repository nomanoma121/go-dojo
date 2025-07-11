# CLAUDE.md
必ず日本語で回答してください

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is `go-dojo`, a comprehensive 60-day Go programming curriculum designed to teach professional-level Go development skills. Each day focuses on a specific topic with hands-on exercises, tests, and performance benchmarks.

## Development Commands

### Project Setup
```bash
# Initialize project (already done)
go mod init go-dojo
go mod tidy

# Quick setup with dependencies
make setup
```

### Daily Practice Commands
```bash
# Work on a specific day (e.g., Day 01)
cd day01-context-cancellation
go test -v                    # Run tests
go test -race                 # Check for race conditions
go test -bench=.              # Run benchmarks

# Use Makefile shortcuts
make test-day DAY=01          # Test specific day
make bench-day DAY=01         # Benchmark specific day
make test-all                 # Test all completed days
```

### Development Commands
```bash
# Code quality
make fmt                      # Format code
make vet                      # Static analysis
make lint                     # Lint with golangci-lint

# Testing
go test -race ./...          # Race condition detection
go test -cover ./...         # Coverage analysis
make coverage                # Generate HTML coverage report

# Progress tracking
make progress                # View completion status
```

### Docker Commands (for integration tests)
```bash
make docker-up               # Start test databases
make docker-down             # Stop test containers
```

## Project Architecture

### Directory Structure
- `dayXX-topic-name/`: Individual daily challenges with README, main.go, and main_test.go
- `lib/`: Shared utilities and helpers across multiple days
  - `lib/tester/`: Docker test helpers for database testing
- `tools/`: Development support tools and Makefile
- `progress.csv`: Learning progress tracking

### Daily Challenge Structure
Each day follows a consistent pattern:
- `README.md`: Challenge description, requirements, hints, and scorecard
- `main.go`: Implementation file with TODO comments and function skeletons
- `main_test.go`: Comprehensive tests including edge cases, race conditions, and benchmarks

### Key Patterns and Conventions
- All challenges use table-driven tests
- Race condition testing with `-race` flag
- Performance benchmarking for concurrent code
- Error handling with context awareness
- Resource cleanup with defer statements
- Interface-based design for testability

### Curriculum Progression
1. **Days 1-15**: Advanced concurrency (Context, Mutex, Worker Pools, Pipelines)
2. **Days 16-30**: Production Web APIs (Middleware, Authentication, Testing)
3. **Days 31-45**: Database & Caching (Transactions, Connection Pooling, Redis)
4. **Days 46-60**: Distributed Systems (gRPC, Message Queues, Observability)

## Common Testing Patterns

### Race Condition Testing
Always run with `-race` flag for concurrent code:
```bash
go test -race
```

### Benchmark Testing
Compare performance between implementations:
```bash
go test -bench=. -benchmem
```

### Integration Testing
Use dockertest helpers from `lib/tester/` for database tests.

## Dependencies

Key external dependencies:
- `github.com/ory/dockertest/v3`: Container-based integration testing
- `github.com/lib/pq`: PostgreSQL driver
- `github.com/go-redis/redis/v8`: Redis client (used in later challenges)

## Development Guidelines

- Follow TDD: Write tests first, then implement
- Use meaningful variable names and comments
- Handle errors explicitly, avoid panic in production code
- Use context.Context for cancellation and timeouts
- Prefer interfaces for testing and modularity
- Always clean up resources (defer statements)
- Write benchmarks for performance-critical code
