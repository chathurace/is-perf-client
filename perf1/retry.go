package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// readFailedUsersFromCSV reads failed users from the CSV file
func (te *TestExecutor) readFailedUsersFromCSV() ([]FailedUser, error) {
	file, err := os.Open(te.config.Execution.FailedUsersCsvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open failed users CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %v", err)
	}

	var failedUsers []FailedUser
	
	// Skip header row if exists
	startRow := 0
	if len(records) > 0 && (records[0][0] == "TenantID" || records[0][0] == "Tenant ID") {
		startRow = 1
	}
	
	for i := startRow; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 {
			continue // Skip malformed records
		}

		tenantID, err := strconv.Atoi(record[0])
		if err != nil {
			fmt.Printf("Warning: Invalid tenant ID in CSV: %s\n", record[0])
			continue
		}

		failedUsers = append(failedUsers, FailedUser{
			TenantID:  tenantID,
			Username:  record[1],
			Error:     record[2],
			Timestamp: record[3],
		})
	}

	return failedUsers, nil
}

// ExecuteRetryFailed retries only the failed users from the CSV file
func (te *TestExecutor) ExecuteRetryFailed() error {
	fmt.Println("Starting retry of failed users...")
	
	// Create failed users writer in append mode for logging new failures during retry
	failedUsersWriter, err := NewFailedUsersCSVWriterAppend(te.config.Execution.FailedUsersCsvPath)
	if err != nil {
		return fmt.Errorf("failed to create failed users CSV writer: %v", err)
	}
	defer failedUsersWriter.Close()
	
	// Temporarily assign the writer to the executor for use in retry workers
	te.failedUsersWriter = failedUsersWriter
	
	// Read failed users from CSV
	failedUsers, err := te.readFailedUsersFromCSV()
	if err != nil {
		return fmt.Errorf("failed to read failed users: %v", err)
	}
	
	if len(failedUsers) == 0 {
		fmt.Println("No failed users found to retry.")
		return nil
	}
	
	fmt.Printf("Found %d failed users to retry\n", len(failedUsers))
	
	startTime := time.Now()
	
	// Calculate users per thread using configured number of threads
	usersPerThread := len(failedUsers) / te.config.Execution.NoOfThreads
	remainingUsers := len(failedUsers) % te.config.Execution.NoOfThreads
	
	// Create retry worker tasks
	var retryTasks []RetryWorkerTask
	userStart := 0
	
	for threadID := 0; threadID < te.config.Execution.NoOfThreads; threadID++ {
		threadUsers := usersPerThread
		if threadID < remainingUsers {
			threadUsers++ // Distribute remaining users to first few threads
		}
		
		if threadUsers > 0 {
			userEnd := userStart + threadUsers - 1
			
			// Create a separate HTTP client for this retry task
			taskClient := NewHTTPClient(te.config)
			
			retryTasks = append(retryTasks, RetryWorkerTask{
				ThreadID:    threadID,
				UserStart:   userStart,
				UserEnd:     userEnd,
				FailedUsers: failedUsers,
				Client:      taskClient,
			})
			userStart = userEnd + 1
		}
	}
	
	// Create wait group and result channel
	var wg sync.WaitGroup
	resultChan := make(chan TestResult, len(failedUsers))
	
	// Start result processor
	go te.processResults(resultChan)
	
	// Apply ramp-up delay between thread starts
	rampUpDelay := time.Duration(te.config.Execution.RampUpPeriod) * time.Second / time.Duration(te.config.Execution.NoOfThreads)
	
	// Start retry worker goroutines
	for _, task := range retryTasks {
		wg.Add(1)
		go te.retryUsersWorkerScalable(task, resultChan, &wg)
		
		// Ramp-up delay
		if rampUpDelay > 0 {
			time.Sleep(rampUpDelay)
		}
	}
	
	// Wait for all workers to complete
	wg.Wait()
	close(resultChan)
	
	duration := time.Since(startTime)
	fmt.Printf("\nRetry execution completed in %v\n", duration)
	
	// Print statistics
	te.stats.PrintStats()
	
	return nil
}

// retryUsersWorkerScalable retries a chunk of failed users assigned to a specific thread
func (te *TestExecutor) retryUsersWorkerScalable(task RetryWorkerTask, resultChan chan<- TestResult, wg *sync.WaitGroup) {
	defer wg.Done()
	
	usersToRetry := task.FailedUsers[task.UserStart:task.UserEnd+1]
	fmt.Printf("Thread %d: Retrying %d users (indices %d-%d)\n", task.ThreadID, len(usersToRetry), task.UserStart, task.UserEnd)
	
	for _, user := range usersToRetry {
		result := TestResult{
			TenantIndex: user.TenantID,
			UserIndex:   -1, // We don't have the original user index
			ThreadID:    task.ThreadID,
		}
		
		// Extract user index from username if possible (assuming format like "prefix_index")
		userIndex := -1
		if parts := strings.Split(user.Username, "_"); len(parts) > 1 {
			if idx, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				userIndex = idx
				result.UserIndex = userIndex
			}
		}
		
		userResp, err := task.Client.CreateUserWithName(user.TenantID, user.Username)
		if err != nil {
			result.Success = false
			result.Error = err
			
			// Write failed user to CSV file again
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			if csvErr := te.failedUsersWriter.WriteFailedUser(user.TenantID, user.Username, err.Error(), timestamp); csvErr != nil {
				fmt.Printf("Thread %d: Failed to write failed user to CSV: %v\n", task.ThreadID, csvErr)
			}
			
			fmt.Printf("Thread %d: Failed to retry user %s for tenant %d: %v\n", 
				task.ThreadID, user.Username, user.TenantID, err)
		} else {
			result.Success = true
			result.ScimID = userResp.ID
			fmt.Printf("Thread %d: Successfully retried user %s for tenant %d with SCIM ID: %s\n", 
				task.ThreadID, user.Username, user.TenantID, userResp.ID)
		}
		
		resultChan <- result
	}
	
	fmt.Printf("Thread %d: Completed retry for %d users\n", task.ThreadID, len(usersToRetry))
}