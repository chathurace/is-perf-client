package main

import (
	"fmt"
	"time"
)

// TestExecutor handles the execution of the SCIM2 test
type TestExecutor struct {
	config            *Config
	csvWriter         *CSVWriter
	failedUsersWriter *FailedUsersCSVWriter
	stats             *TestStats
}

// NewTestExecutor creates a new test executor
func NewTestExecutor(config *Config, retryMode bool) (*TestExecutor, error) {
	csvWriter, err := NewCSVWriter(config.Execution.ScimIdCsvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSV writer: %v", err)
	}
	
	var failedUsersWriter *FailedUsersCSVWriter
	
	// Only create failed users writer if NOT in retry mode (to avoid truncating existing file)
	if !retryMode {
		failedUsersWriter, err = NewFailedUsersCSVWriter(config.Execution.FailedUsersCsvPath)
		if err != nil {
			csvWriter.Close() // Clean up the first writer if second fails
			return nil, fmt.Errorf("failed to create failed users CSV writer: %v", err)
		}
	}
	
	stats := NewTestStats()
	
	return &TestExecutor{
		config:            config,
		csvWriter:         csvWriter,
		failedUsersWriter: failedUsersWriter,
		stats:             stats,
	}, nil
}

// Close cleans up resources
func (te *TestExecutor) Close() error {
	var err1, err2 error
	if te.csvWriter != nil {
		err1 = te.csvWriter.Close()
	}
	if te.failedUsersWriter != nil {
		err2 = te.failedUsersWriter.Close()
	}
	
	if err1 != nil {
		return err1
	}
	return err2
}

// Execute runs the complete test execution
func (te *TestExecutor) Execute() error {
	fmt.Printf("Starting SCIM2 test execution with config:\n")
	fmt.Printf("- Threads: %d\n", te.config.Execution.NoOfThreads)
	fmt.Printf("- Users: %d\n", te.config.Execution.NoOfUsers)
	fmt.Printf("- User Start Number: %d\n", te.config.Execution.UserStartNumber)
	fmt.Printf("- Tenants: %d\n", te.config.Execution.NoOfTenants)
	fmt.Printf("- Tenant Start Number: %d\n", te.config.Execution.TenantStartNumber)
	fmt.Printf("- Server: %s\n", te.config.GetServerURL())
	fmt.Println()
	
	startTime := time.Now()
	
	// Phase 1: Create roles
	if err := te.ExecuteRoleCreation(); err != nil {
		return fmt.Errorf("role creation failed: %v", err)
	}
	
	// Phase 2: Create users
	if err := te.ExecuteUserCreation(); err != nil {
		return fmt.Errorf("user creation failed: %v", err)
	}
	
	duration := time.Since(startTime)
	fmt.Printf("\nTest execution completed in %v\n", duration)
	
	// Print statistics
	te.stats.PrintStats()
	
	return nil
}
