package redis

// RedisDatabaseFields defines metadata fields for Redis databases
// +marmot:metadata
type RedisDatabaseFields struct {
	Host             string `json:"host" metadata:"host" description:"Redis server hostname"`
	Port             int    `json:"port" metadata:"port" description:"Redis server port"`
	Database         string `json:"database" metadata:"database" description:"Database name (e.g. db0)"`
	RedisVersion     string `json:"redis_version" metadata:"redis_version" description:"Redis server version"`
	Role             string `json:"role" metadata:"role" description:"Replication role (master/slave)"`
	UptimeSeconds    string `json:"uptime_seconds" metadata:"uptime_seconds" description:"Server uptime in seconds"`
	ConnectedClients string `json:"connected_clients" metadata:"connected_clients" description:"Number of connected clients"`
	UsedMemoryHuman  string `json:"used_memory_human" metadata:"used_memory_human" description:"Human-readable used memory"`
	MaxmemoryPolicy  string `json:"maxmemory_policy" metadata:"maxmemory_policy" description:"Eviction policy when maxmemory is reached"`
	KeyCount         int64  `json:"key_count" metadata:"key_count" description:"Number of keys in the database"`
	ExpiresCount     int64  `json:"expires_count" metadata:"expires_count" description:"Number of keys with an expiration"`
	AvgTTLMs         int64  `json:"avg_ttl_ms" metadata:"avg_ttl_ms" description:"Average TTL in milliseconds"`
}
