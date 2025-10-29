package main

// WorkerTask represents a task for a worker thread
type WorkerTask struct {
	UserStart   int
	UserEnd     int
	ThreadID    int
	Client      *HTTPClient
}

// RetryWorkerTask represents a task for retry worker thread
type RetryWorkerTask struct {
	ThreadID    int
	UserStart   int
	UserEnd     int
	FailedUsers []FailedUser
	Client      *HTTPClient
}

// FailedUser represents a failed user from CSV
type FailedUser struct {
	TenantID  int
	Username  string
	Error     string
	Timestamp string
}