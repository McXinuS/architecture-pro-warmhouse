# Smart Home Sensor Management API

## Prerequisites

- Docker and Docker Compose

## Getting Started

### Option 1: Using Docker Compose (Recommended)

The easiest way to start the application is to use Docker Compose:

```bash
./init.sh
```

This script will:

1. Build and start all containers (PostgreSQL, RabbitMQ, temperature-api, the monolith and the new microservices)
2. Wait for the services to be ready
3. Display information about how to access the API

Alternatively, you can run Docker Compose directly:

```bash
docker compose up --build -d
```

| Service | URL |
|---|---|
| Monolith (`smart_home`) | http://localhost:8080 |
| `temperature_api` | http://localhost:8081 |
| `device_service` (Kotlin) | http://localhost:8082 |
| `telemetry_service` (Go) | http://localhost:8083 |
| RabbitMQ management | http://localhost:15672 (`warmhouse` / `warmhouse`) |

### Option 2: Manual setup

If you prefer to run the application without Docker:

1. Start the PostgreSQL database:

```bash
docker compose up -d postgres
```

2. Build and run the application:

```bash
go build -o smarthome
./smarthome
```

## API Testing

A Postman collection is provided for testing the API. Import the `smarthome-api.postman_collection.json` file into Postman to get started.

## API Endpoints

Monolith:

- `GET /health` - Health check
- `GET /api/v1/sensors` - Get all sensors
- `GET /api/v1/sensors/:id` - Get a specific sensor
- `POST /api/v1/sensors` - Create a new sensor
- `PUT /api/v1/sensors/:id` - Update a sensor
- `DELETE /api/v1/sensors/:id` - Delete a sensor
- `PATCH /api/v1/sensors/:id/value` - Update a sensor's value and status

Temperature API:

- `GET /temperature?location=` - Random temperature for a room
- `GET /temperature/:sensorId` - Random temperature for a sensor

Device Service:

- `GET /api/v1/devices[?type=]` - List devices
- `GET /api/v1/devices/:id` - Get a device
- `POST /api/v1/devices` - Register a device (`name`, `typeCode`, `location`)
- `POST /api/v1/devices/:id/commands` - Send a command (`name`, `params`, `source`), returns `202`

Telemetry Service:

- `GET /api/v1/telemetry/devices/:deviceId/latest` - Latest value of every metric
- `GET /api/v1/telemetry/devices/:deviceId/history?metric=&from=&to=&interval=` - Metric history (`raw`, `1m`, `1h`, `1d`)
