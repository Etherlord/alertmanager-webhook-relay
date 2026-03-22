package notify

// SendResult represents the outcome of a single notification delivery attempt.
// Workers send these to the dispatcher via a result channel.
type SendResult struct {
	GroupKey string
	Channel  string
	Err      error
}
