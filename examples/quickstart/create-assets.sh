#!/bin/sh
set -e

echo "Installing required packages..."
apk add --no-cache curl jq

echo "Using Marmot API at: $MARMOT_API_URL"

sleep 5

# Login using default user/pass
echo "Logging in with admin user..."
LOGIN_RESPONSE=$(curl -s -X POST "$MARMOT_API_URL/users/login" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin"
  }')

# Get token
ACCESS_TOKEN=$(echo $LOGIN_RESPONSE | busybox grep -o '"access_token":"[^"]*"' | busybox cut -d'"' -f4)
if [ -z "$ACCESS_TOKEN" ]; then
  echo "Failed to get access token. Login response: $LOGIN_RESPONSE"
  exit 1
fi
echo "Successfully obtained access token"

# Create API key
echo "Creating API key..."
API_KEY_RESPONSE=$(curl -s -X POST "$MARMOT_API_URL/users/apikeys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{
    "name": "quickstart-key"
  }')

API_KEY=$(echo $API_KEY_RESPONSE | busybox grep -o '"key":"[^"]*"' | busybox cut -d'"' -f4)
if [ -z "$API_KEY" ]; then
  echo "Failed to create API key. Response: $API_KEY_RESPONSE"
  exit 1
fi
echo "Successfully created API key"

create_asset() {
  name=$1
  type=$2
  description=$3
  services=$4
  tags=$5
  metadata=$6
  schema=$7
  environments=$8
  
  echo "Creating asset: $name ($type)..."
  
  if [ -z "$schema" ] && [ -z "$environments" ]; then
    request_data="{
      \"name\": \"$name\",
      \"type\": \"$type\",
      \"description\": \"$description\",
      \"providers\": $services,
      \"tags\": $tags,
      \"metadata\": $metadata
    }"
  elif [ -z "$environments" ]; then
    request_data="{
      \"name\": \"$name\",
      \"type\": \"$type\",
      \"description\": \"$description\",
      \"providers\": $services,
      \"tags\": $tags,
      \"metadata\": $metadata,
      \"schema\": $schema
    }"
  elif [ -z "$schema" ]; then
    request_data="{
      \"name\": \"$name\",
      \"type\": \"$type\",
      \"description\": \"$description\",
      \"providers\": $services,
      \"tags\": $tags,
      \"metadata\": $metadata,
      \"environments\": $environments
    }"
  else
    request_data="{
      \"name\": \"$name\",
      \"type\": \"$type\",
      \"description\": \"$description\",
      \"providers\": $services,
      \"tags\": $tags,
      \"metadata\": $metadata,
      \"schema\": $schema,
      \"environments\": $environments
    }"
  fi
  
  RESPONSE=$(curl -s -X POST "$MARMOT_API_URL/assets" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
    -d "$request_data")
  
  if echo "$RESPONSE" | busybox grep -q "error"; then
    echo "Error creating asset: $RESPONSE"
    return 1
  fi
    
  echo "Created asset: $name"
}

create_lineage() {
  source_type=$1
  source_name=$2
  target_type=$3
  target_name=$4
  
  echo "Creating lineage from $source_name to $target_name..."
  
  source_asset=$(curl -s -X GET "$MARMOT_API_URL/assets/lookup/$source_type/$source_name" \
    -H "X-API-Key: $API_KEY")
  source_mrn=$(echo $source_asset | busybox grep -o '"mrn":"[^"]*"' | busybox cut -d'"' -f4)
  
  if [ -z "$source_mrn" ]; then
    echo "Failed to get source asset MRN for $source_name"
    return 1
  fi
  
  target_asset=$(curl -s -X GET "$MARMOT_API_URL/assets/lookup/$target_type/$target_name" \
    -H "X-API-Key: $API_KEY")
  target_mrn=$(echo $target_asset | busybox grep -o '"mrn":"[^"]*"' | busybox cut -d'"' -f4)
  
  if [ -z "$target_mrn" ]; then
    echo "Failed to get target asset MRN for $target_name"
    return 1
  fi
  
  RESPONSE=$(curl -s -X POST "$MARMOT_API_URL/lineage/direct" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
    -d "{
      \"source\": \"$source_mrn\",
      \"target\": \"$target_mrn\"
    }")
  
  if echo "$RESPONSE" | busybox grep -q "error"; then
    echo "Error creating lineage: $RESPONSE"
    return 1
  fi
    
  echo "Created lineage from $source_name to $target_name"
}

echo "Starting asset creation..."

# Define schema as object, then stringify it
kafka_schema_obj='{
  "message": {
    "type": "object",
    "required": ["event_id", "customer_id", "event_type", "timestamp"],
    "properties": {
      "event_id": {
        "type": "string",
        "description": "Unique identifier for the event"
      },
      "customer_id": {
        "type": "string",
        "description": "Customer identifier"
      },
      "event_type": {
        "type": "string",
        "enum": ["SIGN_UP", "LOGIN", "PURCHASE", "ACCOUNT_UPDATE", "LOGOUT"],
        "description": "Type of customer event"
      },
      "timestamp": {
        "type": "integer",
        "format": "int64",
        "description": "Event timestamp in milliseconds since epoch"
      },
      "properties": {
        "type": "object",
        "description": "Additional event properties",
        "properties": {
          "session_id": {
            "type": "string",
            "description": "Session identifier"
          },
          "source": {
            "type": "string",
            "description": "Source of the event"
          },
          "device_info": {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "description": "Device type"
              },
              "os": {
                "type": "string",
                "description": "Operating system"
              }
            }
          }
        }
      }
    },
    "example": {
      "event_id": "evt_12345678",
      "customer_id": "cust_87654321",
      "event_type": "PURCHASE",
      "timestamp": 1648208912000,
      "properties": {
        "session_id": "sess_abc123",
        "source": "web",
        "device_info": {
          "type": "desktop",
          "os": "macos"
        }
      }
    }
  },
  "headers": {
    "type": "object",
    "properties": {
      "trace_id": {
        "type": "string",
        "description": "Distributed tracing identifier"
      },
      "content_type": {
        "type": "string",
        "description": "Content type of the message"
      }
    },
    "example": {
      "trace_id": "trace_987654321",
      "content_type": "application/json"
    }
  }
}'

# Create separate schema strings for message and headers
kafka_message_schema=$(echo "$kafka_schema_obj" | jq -c '.message' | sed 's/"/\\"/g')
kafka_headers_schema=$(echo "$kafka_schema_obj" | jq -c '.headers' | sed 's/"/\\"/g')
kafka_schema='{"message": "'"$kafka_message_schema"'", "headers": "'"$kafka_headers_schema"'"}'

kafka_environments='{
  "dev": {
    "name": "Development",
    "path": "dev-customer-events",
    "metadata": {
      "retention_ms": "86400000",
      "auto_create_topics": "true",
      "cleanup_policy": "delete",
      "min_insync_replicas": "1"
    }
  },
  "prod": {
    "name": "Production",
    "path": "prod-customer-events",
    "metadata": {
      "retention_ms": "604800000",
      "auto_create_topics": "false",
      "cleanup_policy": "compact,delete",
      "min_insync_replicas": "2",
      "monitoring_enabled": "true"
    }
  }
}'

create_asset \
  "customer-events-stream" \
  "Topic" \
  "Kafka stream for customer events" \
  "[\"Kafka\"]" \
  "[\"events\", \"streaming\", \"real-time\", \"customer-data\"]" \
  "{\"owner\": \"platform-team\", \"partitions\": \"24\", \"replication_factor\": \"3\"}" \
  "$kafka_schema" \
  "$kafka_environments"

# Define postgres schema object
postgres_schema_obj='{
  "message": {
    "type": "object",
    "properties": {
      "tables": {
        "type": "array",
        "items": {
          "type": "object",
          "required": ["name", "columns"],
          "properties": {
            "name": {
              "type": "string",
              "description": "Table name"
            },
            "description": {
              "type": "string",
              "description": "Table description"
            },
            "columns": {
              "type": "array",
              "items": {
                "type": "object",
                "required": ["name", "type"],
                "properties": {
                  "name": {
                    "type": "string",
                    "description": "Column name"
                  },
                  "type": {
                    "type": "string",
                    "description": "Column data type"
                  },
                  "description": {
                    "type": "string",
                    "description": "Column description"
                  },
                  "nullable": {
                    "type": "boolean",
                    "description": "Whether the column can be null"
                  },
                  "primary_key": {
                    "type": "boolean",
                    "description": "Whether the column is part of the primary key"
                  }
                }
              }
            }
          }
        }
      }
    },
    "example": {
      "tables": [
        {
          "name": "customers",
          "description": "Customer records",
          "columns": [
            {"name": "id", "type": "uuid", "description": "Primary identifier", "nullable": false, "primary_key": true},
            {"name": "email", "type": "varchar(255)", "description": "Customer email address", "nullable": false},
            {"name": "created_at", "type": "timestamp", "description": "Creation timestamp", "nullable": false}
          ]
        },
        {
          "name": "orders",
          "description": "Customer orders",
          "columns": [
            {"name": "id", "type": "uuid", "description": "Primary identifier", "nullable": false, "primary_key": true},
            {"name": "customer_id", "type": "uuid", "description": "Reference to customers.id", "nullable": false},
            {"name": "total", "type": "decimal(10,2)", "description": "Order total", "nullable": false}
          ]
        }
      ]
    }
  }
}'

# Convert postgres schema to schema format expected by API
postgres_schema_str=$(echo "$postgres_schema_obj" | jq -c '.message' | sed 's/"/\\"/g')
postgres_schema='{"message": "'"$postgres_schema_str"'"}'

postgres_environments='{
  "dev": {
    "name": "Development",
    "path": "dev-db/customer_data",
    "metadata": {
      "backup_frequency": "daily",
      "connection_string": "postgresql://dev-user:******@dev-db:5432/customer_data",
      "max_connections": "20"
    }
  },
  "prod": {
    "name": "Production",
    "path": "prod-db/customer_data",
    "metadata": {
      "backup_frequency": "hourly",
      "connection_string": "postgresql://prod-user:******@prod-db:5432/customer_data",
      "ha_enabled": "true",
      "max_connections": "100",
      "monitoring_enabled": "true"
    }
  }
}'

create_asset \
  "customer-data-warehouse" \
  "Database" \
  "PostgreSQL database for customer data" \
  "[\"PostgreSQL\"]" \
  "[\"database\", \"warehouse\", \"structured-data\"]" \
  "{\"owner\": \"data-team\", \"version\": \"14.5\", \"size\": \"medium\"}" \
  "$postgres_schema" \
  "$postgres_environments"

create_asset \
  "customer-data-lake" \
  "Bucket" \
  "S3 bucket for customer data lake storage" \
  "[\"S3\"]" \
  "[\"storage\", \"data-lake\", \"raw-data\"]" \
  "{\"owner\": \"data-platform-team\", \"region\": \"us-west-2\"}" \
  "" ""

create_asset \
  "order-processing-service" \
  "Service" \
  "Microservice for processing customer orders" \
  "[\"Kubernetes\"]" \
  "[\"microservice\", \"orders\", \"processing\"]" \
  "{\"owner\": \"order-team\", \"language\": \"golang\", \"version\": \"1.2.3\"}" \
  "" ""

sleep 2

create_lineage "Topic" "customer-events-stream" "Service" "order-processing-service"
create_lineage "Service" "order-processing-service" "Database" "customer-data-warehouse"
create_lineage "Service" "order-processing-service" "Bucket" "customer-data-lake"

echo "Asset creation completed successfully!"
