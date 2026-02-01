# MarketFlow — Real-Time Market Data Processing System

## Description

MarketFlow is a real-time market data processing system built with **Go**, designed using **Hexagonal Architecture (Ports & Adapters)** and concurrency patterns to ensure scalability, reliability, and clean separation of concerns.

---

## ✨ Functionality

- **Operating Modes**
  - **Live mode** — real market data from exchanges
  - **Test mode** — synthetic data generation

- **Concurrent Processing**
  - Worker Pool
  - Fan-In / Fan-Out patterns

- **Caching**
  - Redis for fast access to the latest prices

- **Data Storage**
  - PostgreSQL for aggregated and historical data

- **REST API**
  - Full set of endpoints for retrieving market data

- **Graceful Shutdown**
  - Safe and controlled shutdown of all services and goroutines

---

## 🏗️ Architecture

The project follows **Hexagonal Architecture (Ports & Adapters)** principles:

- **Domain Layer**
  - Core business logic and domain models

- **Application Layer**
  - Use cases and orchestration logic

- **Adapters**
  - External integrations (PostgreSQL, Redis, HTTP API, Exchanges)

---

## 🚀 Installation & Running

### Install dependencies

```bash
go mod tidy
Set up infrastructure
make docker-setup
Build the project
make build
Run the application
./marketflow --port 8080
📡 API Endpoints
Prices
GET /prices/latest/{symbol}
Get the latest price

GET /prices/latest/{exchange}/{symbol}
Get the latest price from a specific exchange

GET /prices/highest/{symbol}?period=1m
Get the highest price for a given period

GET /prices/lowest/{symbol}?period=1m
Get the lowest price for a given period

GET /prices/average/{symbol}?period=1m
Get the average price for a given period

Mode Management
POST /mode/test
Switch to test mode (synthetic data)

POST /mode/live
Switch to live market data mode

System Health
GET /health
System health status

⚙️ Concurrency Patterns
Generator

Generates synthetic market data in test mode

Fan-Out

Distributes updates across multiple workers

Worker Pool

Dedicated worker pools (5 workers per exchange)

Fan-In

Aggregates data from all sources into a unified stream

⚙️ Configuration
The system is configured using a config.yaml file:

server:
  port: 8080

database:
  host: localhost
  port: 5432
  user: marketflow
  password: password
  dbname: marketflow
  sslmode: disable

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
  ttl: 300s

exchanges:
  - name: "exchange1"
    host: "127.0.0.1"
    port: 40101
🧠 Implementation Details
Failover

Automatic reconnection to exchanges

Batching

Batch inserts for efficient database operations

Fallback Strategy

PostgreSQL continues to operate even if Redis is unavailable

Data Cleanup

Automatic eviction of expired cache entries

Graceful Shutdown

Proper shutdown of all running goroutines

