#!/bin/bash

# Exit on any error
set -e

cd "$(dirname "$0")"

# Docker Compose v2 is a docker plugin; fall back to the standalone v1 binary.
if docker compose version > /dev/null 2>&1; then
  COMPOSE="docker compose"
else
  COMPOSE="docker-compose"
fi

echo "Starting the Smart Home Sensor API..."
echo "Building and starting containers..."
$COMPOSE up --build -d

echo "Waiting for services to be ready..."
# Wait for PostgreSQL to be ready
for i in {1..30}; do
  if docker exec smarthome-postgres pg_isready -h 127.0.0.1 -U postgres > /dev/null 2>&1; then
    echo "PostgreSQL is ready!"
    break
  fi
  echo "Waiting for PostgreSQL to start... ($i/30)"
  sleep 1
done

# Check if PostgreSQL is ready
if ! docker exec smarthome-postgres pg_isready -h 127.0.0.1 -U postgres > /dev/null 2>&1; then
  echo "Error: PostgreSQL did not start within the expected time."
  exit 1
fi

echo "All services are up and running!"
echo "Monolith API:          http://localhost:8080"
echo "Temperature API:       http://localhost:8081"
echo "Device Service:        http://localhost:8082"
echo "Telemetry Service:     http://localhost:8083"
echo "RabbitMQ management:   http://localhost:15672 (warmhouse / warmhouse)"
echo ""
echo "To view logs, run: $COMPOSE logs -f"
echo "To stop the services, run: $COMPOSE down"
