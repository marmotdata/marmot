#!/bin/bash
set -e

echo "Waiting for PostgreSQL..."
python /usr/app/wait-for-postgres.py

echo "Testing connectivity to Marmot..."
curl -v http://192.168.0.9:8080 || echo "Connection failed"

echo "Starting dbt with OpenLineage..."
cd /usr/app/dbt_project

# Set OpenLineage environment variables  
export OPENLINEAGE_URL=http://192.168.0.9:8080
export OPENLINEAGE_NAMESPACE=dbt_test_project

# Install dependencies
dbt deps

# Try manual OpenLineage test first
echo "Testing OpenLineage manually..."
cat > test_event.json << EOF
{
  "eventType": "START",
  "eventTime": "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)",
  "run": {"runId": "test-run-123"},
  "job": {"namespace": "test", "name": "test-job"},
  "producer": "manual-test"
}
EOF

curl -X POST http://192.168.0.9:8080/api/v1/lineage \
  -H "Content-Type: application/json" \
  -d @test_event.json || echo "Manual test failed"

# Run with debug logging
echo "Running dbt with OpenLineage integration..."
dbt-ol run --profiles-dir . --debug
