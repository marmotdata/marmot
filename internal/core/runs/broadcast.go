package runs

// EventBroadcaster defines the interface for broadcasting job run events
type EventBroadcaster interface {
	BroadcastJobRunCreated(run *JobRun)
	BroadcastJobRunClaimed(run *JobRun)
	BroadcastJobRunStarted(run *JobRun)
	BroadcastJobRunProgress(run *JobRun)
	BroadcastJobRunCompleted(run *JobRun)
	BroadcastJobRunCancelled(run *JobRun)
}

// NoopBroadcaster is a broadcaster that does nothing (for when websockets are disabled)
type NoopBroadcaster struct{}

func (n *NoopBroadcaster) BroadcastJobRunCreated(run *JobRun)   {}
func (n *NoopBroadcaster) BroadcastJobRunClaimed(run *JobRun)   {}
func (n *NoopBroadcaster) BroadcastJobRunStarted(run *JobRun)   {}
func (n *NoopBroadcaster) BroadcastJobRunProgress(run *JobRun)  {}
func (n *NoopBroadcaster) BroadcastJobRunCompleted(run *JobRun) {}
func (n *NoopBroadcaster) BroadcastJobRunCancelled(run *JobRun) {}
