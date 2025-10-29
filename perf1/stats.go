package main

import (
	"fmt"
	"sync"
)

// TestResult holds the result of a test operation
type TestResult struct {
	TenantIndex    int
	UserIndex      int
	Success        bool
	ScimID         string
	Error          error
	ThreadID       int
}

// TestStats holds statistics about test execution
type TestStats struct {
	TotalUsers    int
	SuccessUsers  int
	FailedUsers   int
	TotalRoles    int
	SuccessRoles  int
	FailedRoles   int
	mutex         sync.Mutex
}

// NewTestStats creates a new TestStats instance
func NewTestStats() *TestStats {
	return &TestStats{}
}

// IncrementRole increments role creation statistics
func (ts *TestStats) IncrementRole(success bool) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	
	ts.TotalRoles++
	if success {
		ts.SuccessRoles++
	} else {
		ts.FailedRoles++
	}
}

// IncrementUser increments user creation statistics
func (ts *TestStats) IncrementUser(success bool) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	
	ts.TotalUsers++
	if success {
		ts.SuccessUsers++
	} else {
		ts.FailedUsers++
	}
}

// PrintStats prints the current statistics
func (ts *TestStats) PrintStats() {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	
	fmt.Println("\n=== Test Execution Statistics ===")
	fmt.Printf("Roles - Total: %d, Success: %d, Failed: %d\n", 
		ts.TotalRoles, ts.SuccessRoles, ts.FailedRoles)
	fmt.Printf("Users - Total: %d, Success: %d, Failed: %d\n", 
		ts.TotalUsers, ts.SuccessUsers, ts.FailedUsers)
	
	if ts.TotalRoles > 0 {
		roleSuccessRate := float64(ts.SuccessRoles) / float64(ts.TotalRoles) * 100
		fmt.Printf("Role Success Rate: %.2f%%\n", roleSuccessRate)
	}
	
	if ts.TotalUsers > 0 {
		userSuccessRate := float64(ts.SuccessUsers) / float64(ts.TotalUsers) * 100
		fmt.Printf("User Success Rate: %.2f%%\n", userSuccessRate)
	}
	fmt.Println("================================")
}

// processResults processes test results and updates statistics
func (te *TestExecutor) processResults(resultChan <-chan TestResult) {
	for result := range resultChan {
		te.stats.IncrementUser(result.Success)
		
		// if result.Success && result.ScimID != "" {
		// 	if err := te.csvWriter.WriteScimID(result.ScimID); err != nil {
		// 		fmt.Printf("Failed to write SCIM ID to CSV: %v\n", err)
		// 	}
		// }
	}
}