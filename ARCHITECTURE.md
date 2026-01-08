# Code Grading System Architecture

## Overview

A distributed code grading system consisting of 3 core services that work together to execute user code and grade it against test cases.

## Services

### 1. Main Service (API Gateway)
- **Role**: Entry point for user requests
- **Responsibilities**:
  - Accept user code submissions
  - Route requests to Grader Service via gRPC
  - Return results to users

### 2. Grader Service
Consists of two components:

#### Grader Master (gRPC Server)
- **Role**: Coordinate grading jobs
- **Responsibilities**:
  - Expose gRPC endpoints (`Run`, `Grade`, `GenerateTestCases`)
  - Enqueue jobs to RabbitMQ for workers to consume
  - Stream results back to Main Service
  - Store grade results to database
  - Handle testcase generation requests from Task Service

#### Grader Worker
- **Role**: Execute code and perform grading
- **Responsibilities**:
  - Fetch runners and comparison scripts from Config Service at startup
  - Consume `run` and `grade` queues from RabbitMQ
  - Execute user code using isolate (sandboxed execution)
  - Compare output against expected results using comparison scripts
  - Publish results back via RabbitMQ

### 3. Task Service
- **Role**: Store task metadata and testcases
- **Responsibilities**:
  - Store tasks, testcases, solution code, allowed runners
  - Provide task information via gRPC
  - Cache testcases in Redis to reduce DB load
  - Trigger testcase regeneration when solution changes

### 4. Config Service (Additional)
- **Role**: Store configuration for runners and comparison scripts
- **Responsibilities**:
  - Store runner configurations (Python, C, C++, etc.)
  - Store comparison scripts (user-provided comparison logic)
  - Provide configurations via gRPC

## Communication Flow

### Grading Flow
```
Main Service → gRPC → Grader Master → RabbitMQ (grade queue) → Grader Worker
                                                                 ↓
                                        Grader Worker → Config Service (compare script, at startup)
                                         Grader Worker → Task Service (testcases)
                                         Grader Worker → Compare (execute)
                                         Grader Worker → RabbitMQ (grade_results)
                                               ↓
                                        Grader Master ← Consume grade_results
                                               ↓
                                        Main Service (return result)
```

### Testcase Generation Flow
```
Admin → Task Service (update solution)
Task Service → gRPC → Grader Master (GenerateTestCases)
Grader Master → RabbitMQ (run queue) → Grader Worker
Grader Worker → Execute solution → Return outputs
Grader Master → Collect outputs → Task Service
Task Service → Store new testcases (Redis cached)
```

## Data Stores

| Service | Data | Storage |
|---------|------|---------|
| Grader Service | Grade results | PostgreSQL (via sqlx) |
| Task Service | Tasks, testcases | PostgreSQL + Redis cache |
| Config Service | Runners, compare scripts | PostgreSQL |
| RabbitMQ | Job queues, results | Message broker |

## Current Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Main Service                                   │
│                              (API Gateway)                                  │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Grader Master                                     │
│                              (gRPC Server)                                  │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ Endpoints:                                                          │   │
│  │   - Run(code, runner) → stream results                              │   │
│  │   - Grade(code, task_id) → execution_id                             │   │
│  │   - GenerateTestCases(solution, inputs) → outputs                   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                        │                                    │
│                    ┌───────────────────┴───────────────────┐                │
│                    ▼                                       ▼                │
│           ┌────────────────┐                      ┌────────────────┐        │
│           │  RabbitMQ: run │                      │ RabbitMQ: grade│        │
│           └────────────────┘                      └────────────────┘        │
└─────────────────────────────────────────────────────────────────────────────┘
                    │                                       │
                    ▼                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            Grader Workers                                   │
│                          (Multiple instances)                               │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ On Startup:                                                         │   │
│  │   - Fetch runners from Config Service                              │   │
│  │   - Fetch compare scripts from Config Service                      │   │
│  │   - Compile comparison scripts                                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ Runtime:                                                            │   │
│  │   - Consume from run/grade queues                                  │   │
│  │   - Execute code in isolate sandbox                                │   │
│  │   - Fetch testcases from Task Service (Redis cached)               │   │
│  │   - Run comparison script                                          │   │
│  │   - Publish results to RabbitMQ                                    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Config Service│    │  Task Service   │    │  PostgreSQL     │
│  (Runners/      │    │  (Tasks/        │    │  (Results)      │
│   Compares)     │    │   Testcases)    │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                        │
         │                        ▼
         │               ┌─────────────────┐
         │               │   Redis         │
         │               │   (Testcase     │
         │               │    Cache)       │
         │               └─────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Admin Interface                                │
│                    (Update solutions, trigger regrades)                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Known Issues & Circular Dependencies

### Circular Dependency
```
Task Service ←→ Grader Service

Task Service → Grader Service:
  - Generate testcases when solution is updated

Grader Service → Task Service:
  - Fetch testcases during grading
  - Fetch compare scripts (via config, but task references them)
```

### Current Mitigation
1. **Redis Caching**: Task Service caches testcases in Redis to reduce DB load and latency
2. **Startup Fetch**: Workers fetch comparison scripts from Config Service at startup (not per-request)
3. **Fanout Restart**: Config Service publishes fanout message when scripts change; workers restart

### Unresolved Concerns
1. **Runtime Dependency**: Grader still depends on Task Service at runtime for testcases
2. **Worker Restart Impact**: Restarting workers drops in-progress tasks
3. **Failure Handling**: No circuit breaker if Task Service is unavailable during grading
4. **Re-grading Workflow**: Not fully implemented - need to track which submissions to re-grade

## Alternative Designs

### Option 1: Async Event-Based (Recommended for Breaking Circular Dependency)

```
Admin → Task Service (update solution)
         │
         ▼
Task Service → RabbitMQ (topic: solution_updated)
                              │
                              ├────→ Grader Service (consume, generate testcases)
                              │           │
                              │           ▼
                              │     RabbitMQ (testcases_ready)
                              │           │
                              └───────────┴────→ Task Service (store testcases)
```

**Pros**:
- Breaks synchronous circular dependency
- More resilient to failures
- Better scalability

**Cons**:
- More complex event flow
- Harder to debug

### Option 2: Pre-fetch and Cache Testcases

```
Main Service → Task Service (fetch task, testcases)
         │
         ▼
Main Service → Grader Service (submit code + testcases)
```

**Pros**:
- Grader doesn't need Task Service at runtime
- Simpler debugging

**Cons**:
- Larger payloads over network
- Main Service needs task info
- Testcases may be stale

### Option 3: Accept the Dependency

Keep current design but add:
- Circuit breakers
- Retry logic with exponential backoff
- Graceful degradation

**Pros**:
- Simpler implementation
- Already functional

**Cons**:
- Tight coupling remains

## Recommendations

### Short Term
1. Add circuit breaker between Grader and Task Service
2. Implement proper error handling when Task Service is unavailable
3. Document the re-grading workflow

### Long Term
1. Consider async event-based approach for testcase generation
2. Implement hot-reload for comparison scripts instead of worker restart
3. Add caching layer in Grader for frequently accessed tasks

## Queue Names

| Queue | Purpose | Publisher | Consumer |
|-------|---------|-----------|----------|
| `run` | Run code without grading | Grader Master | Workers |
| `grade` | Grade code against testcases | Grader Master | Workers |
| `grade_results` | Store grade results | Workers | Grader Master |
| `topic.run_results` | Stream run results | Workers | Grader Master |
| `topic.grade_results` | Stream grade results | Workers | Grader Master |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `ENV` | Environment (development/production) |
| `DATABASE_URL` | PostgreSQL connection string |
| `QUEUE_SERVER_URL` | RabbitMQ connection URL |
| `CONFIG_SERVER_URL` | Config Service gRPC address |
| `TASK_SERVER_URL` | Task Service gRPC address |
| `PORT` | Grader Master gRPC port |

## File Structure

```
go-grader/
├── cmd/
│   ├── master/          # Grader Master entry point
│   └── worker/          # Grader Worker entry point
├── domain/
│   ├── constants/       # Status codes, constants
│   ├── models/          # Data models (Task, TestCase, Result, etc.)
│   ├── services/        # Business logic (Executor, Compare, Isolate, etc.)
│   └── messaging/       # Queue interface
├── internal/
│   ├── adapters/        # Database adapters (sqlx)
│   ├── infrastructure/  # RabbitMQ, Redis clients
│   ├── setup/           # Initialization logic
│   └── logging/         # Logging utilities
├── protos/
│   ├── grader/v1/       # Grader Service protobuf
│   ├── task/v1/         # Task Service protobuf
│   └── config/v1/       # Config Service protobuf
├── configs/             # Configuration loading
├── genproto/            # Generated gRPC code
└── scripts/             # Utility scripts (worker.sh)
```
