package nats

// NATSStreamFields defines metadata fields for NATS JetStream streams
// +marmot:metadata
type NATSStreamFields struct {
	StreamName      string `json:"stream_name" metadata:"stream_name" description:"Name of the JetStream stream"`
	Subjects        string `json:"subjects" metadata:"subjects" description:"Comma-separated list of subjects the stream listens on"`
	RetentionPolicy string `json:"retention_policy" metadata:"retention_policy" description:"Message retention policy (Limits, Interest, WorkQueue)"`
	MaxBytes        int64  `json:"max_bytes" metadata:"max_bytes" description:"Maximum total bytes for the stream (-1 = unlimited)"`
	MaxMsgs         int64  `json:"max_msgs" metadata:"max_msgs" description:"Maximum number of messages (-1 = unlimited)"`
	MaxAge          string `json:"max_age" metadata:"max_age" description:"Maximum age of messages"`
	MaxMsgSize      int64  `json:"max_msg_size" metadata:"max_msg_size" description:"Maximum size of a single message"`
	StorageType     string `json:"storage_type" metadata:"storage_type" description:"Storage backend (File or Memory)"`
	NumReplicas     int    `json:"num_replicas" metadata:"num_replicas" description:"Number of stream replicas"`
	DuplicateWindow string `json:"duplicate_window" metadata:"duplicate_window" description:"Duplicate message tracking window"`
	DiscardPolicy   string `json:"discard_policy" metadata:"discard_policy" description:"Policy when limits are reached (Old or New)"`
	Messages        uint64 `json:"messages" metadata:"messages" description:"Total number of messages in the stream"`
	Bytes           uint64 `json:"bytes" metadata:"bytes" description:"Total bytes stored in the stream"`
	ConsumerCount   int    `json:"consumer_count" metadata:"consumer_count" description:"Number of consumers attached to the stream"`
	FirstSeq        uint64 `json:"first_seq" metadata:"first_seq" description:"Sequence number of the first message"`
	LastSeq         uint64 `json:"last_seq" metadata:"last_seq" description:"Sequence number of the last message"`
	Host            string `json:"host" metadata:"host" description:"NATS server hostname"`
	Port            int    `json:"port" metadata:"port" description:"NATS server port"`
}
