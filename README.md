# Tezos Delegation Service

## Assignment Requirements

### 1. Data Polling Service
- Continuously poll Tezos delegation operations from the [TzKT API](https://api.tzkt.io/#operation/Operations_GetDelegations)
- For each delegation, store:
  - Sender's address (delegator)
  - Timestamp
  - Amount
  - Block height (level)
- Backfill all historical data since Tezos launch (2018)
- Continuously poll for new delegations

### 2. Public API Endpoint
- **Endpoint**: `GET /xtz/delegations`
- **Features**:
  - Filter by year (YYYY) using `?year=` query parameter
  - Pagination (50 items per page) using `?page=`
  - Most recent delegations first
- **Response Format**:
  ```json
  {
    "data": [
      {
        "timestamp": "2022-05-05T06:29:14Z",
        "amount": "125896",
        "delegator": "tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL",
        "level": "2338084"
      },
      {
        "timestamp": "2021-05-07T14:48:07Z",
        "amount": "9856354",
        "delegator": "KT1JejNYjmQYh8yw95u5kfQDRuxJcaUPjUnf",
        "level": "1461334"
      }
    ]
  }
  ```

### 3. Additional Requirements
- Comprehensive test coverage
- Clear local setup instructions
- Simple and maintainable solution
- Production-ready implementation

## 🏗️ Implementation

### Architecture

```
tezos-delegation-service/
├── cmd/
│   └── main.go              # Service entry point & orchestration
├── db/
│   ├── db.go                # Database connection & migrations
│   └── migrations/          # SQL migration files
|   docs/
|   └── insomnia-collection.yaml
├── internal/
│   ├── api/                 # HTTP API handlers & routing
│   ├── config/              # Configuration management
│   ├── poller/              # Background polling service
│   ├── store/               # PostgreSQL data access layer
│   └── tzkt/                # TzKT API client
├── docker-compose.yml       # Local development setup
├── Dockerfile               # Container image definition
└── Makefile                 # Development commands
```

### Key Components

- **Main** (`cmd/main.go`)
  - Application entry point
  - Orchestrates all components (API server, poller, database)
  - Graceful shutdown handling

- **Database** (`db/`)
  - Connection management and migrations
  - PostgreSQL schema with optimized indexes
  - Migration support via golang-migrate

- **API** (`internal/api/`)
  - RESTful HTTP endpoints (`/health`, `/xtz/delegations`)
  - Request validation and error handling
  - CORS middleware and logging

- **Config** (`internal/config/`)
  - Environment-based configuration
  - Database DSN, HTTP port, polling intervals
  - Sensible defaults for local development

- **Poller** (`internal/poller/`)
  - Backfills historical data since 2018
  - Continuously polls for new delegations
  - Idempotent operations with exponential backoff

- **Store** (`internal/store/`)
  - PostgreSQL data access layer
  - Bulk insert operations for efficiency
  - Paginated queries with year filtering

- **TzKT Client** (`internal/tzkt/`)
  - Type-safe TzKT API client
  - Rate limiting (10 req/s) and retry logic
  - Handles network errors gracefully

## Getting Started

### Prerequisites
- Docker and Docker Compose
- Go 1.23+ (this project uses 1.25.1)

### Quick Start

**Step 1: Start the service**
```bash
# Clone or extract the project
cd tezos-delegation-service

# Start all services with Docker Compose
docker compose up --build

# The service will:
# 1. Start PostgreSQL database
# 2. Run database migrations automatically
# 3. Start polling delegations from 2018
# 4. Expose API on http://localhost:8080
```

**Step 2: Wait for initial data (optional)**
```bash
# In another terminal, watch the logs to see data being loaded
docker compose logs -f app

# You should see messages like:
# "poller: inserted 10000 delegations since 2018-06-30T00:00:00Z"
# Wait 10-30 seconds for initial data to load
```

**Step 3: Test the API**
```bash
# Check service health
curl http://localhost:8080/health

# Get latest delegations (50 per page)
curl http://localhost:8080/xtz/delegations

# Filter by year
curl 'http://localhost:8080/xtz/delegations?year=2022'

# Get second page
curl 'http://localhost:8080/xtz/delegations?page=2'

# Combine filters
curl 'http://localhost:8080/xtz/delegations?year=2020&page=1'
```

**Step 4: Stop the service**
```bash
# Stop all services
docker compose down

# Stop and remove all data (including database)
docker compose down -v
```

### Complete Test Sequence

Run this complete sequence to verify everything works:

```bash
# 1. Start the service
docker compose up -d

# 2. Wait for services to be healthy (about 10 seconds)
sleep 10

# 3. Check health
curl -s http://localhost:8080/health | jq

# 4. Get delegations
curl -s http://localhost:8080/xtz/delegations | jq '.data | length'

# 5. Test year filter
curl -s 'http://localhost:8080/xtz/delegations?year=2022' | jq '.data[0]'

# 6. Test pagination
curl -s 'http://localhost:8080/xtz/delegations?page=2' | jq '.data | length'

# 7. View logs
docker compose logs --tail=20 app

# 8. Stop services
docker compose down
```

### Running Tests

```bash
# Run unit tests (no database required)
make test

# Start database for integration tests
make db-up
```

## API Documentation

For a better API testing experience, you can import the [Insomnia collection](docs/insomnia-collection.yaml) located in the `docs` directory.

### `GET /health`

Health check endpoint for monitoring and orchestration tools.

**Response** (200 OK when healthy):
```json
{
  "status": "healthy",
  "checks": {
    "database": "healthy",
    "database_connections": "5 open"
  },
  "uptime": "2h15m30s"
}
```

**Response** (503 Service Unavailable when unhealthy):
```json
{
  "status": "unhealthy",
  "checks": {
    "database": "unhealthy: connection refused"
  },
  "uptime": "5m10s"
}
```

### `GET /xtz/delegations`

**Query Parameters**:
- `year` (optional): Filter by year (YYYY)
- `page` (optional): Page number (default: 1)

**Example Response**:
```json
{
  "data": [
    {
      "timestamp": "2022-05-05T06:29:14Z",
      "amount": "125896",
      "delegator": "tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL",
      "level": "2338084"
    }
  ]
}
```

## Assignment Organisation
I tend to prefer working with dedicated slots when working on take-home assignments. I have mostly organised the time, as follows: 
- Ideation phase - reading requirements, thinking about the structure of the project etc - 40 minutes
- Implementation phase - this is a rough estimate, as I was expected to spend most of the time here - 3 hours
- Testing phase - this covers API testing (e.g. Insomnia API testing, standing up the application and actual tests) - 1 hour

