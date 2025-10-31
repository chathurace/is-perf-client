package main

import (
	"fmt"
	"sync"
	"time"
)

// ExecuteUserCreation creates users using multiple threads
func (te *TestExecutor) ExecuteUserCreation() error {
	fmt.Println("Starting user creation phase...")
	
	// Calculate users per thread
	usersPerThread := te.config.Execution.NoOfUsers / te.config.Execution.NoOfThreads
	remainingUsers := te.config.Execution.NoOfUsers % te.config.Execution.NoOfThreads
	
	// Create worker tasks
	var tasks []WorkerTask
	userStart := te.config.Execution.UserStartNumber
	
	for threadID := 0; threadID < te.config.Execution.NoOfThreads; threadID++ {
		threadUsers := usersPerThread
		if remainingUsers > 0 {
			threadUsers++ // Distribute remaining users to first few threads
			remainingUsers--
		}
		
		userEnd := userStart + threadUsers - 1
		
		// Create a separate HTTP client for this task
		taskClient := NewHTTPClient(te.config)
		
		tasks = append(tasks, WorkerTask{
			UserStart:   userStart,
			UserEnd:     userEnd,
			ThreadID:    threadID,
			Client:      taskClient,
		})
		
		userStart = userEnd + 1
	}
	
	// Create wait group and result channel
	var wg sync.WaitGroup
	totalResults := te.config.Execution.NoOfUsers * te.config.Execution.NoOfTenants
	resultChan := make(chan TestResult, totalResults)
	
	// Start result processor
	go te.processResults(resultChan)
	
	// Apply ramp-up delay between thread starts
	rampUpDelay := time.Duration(te.config.Execution.RampUpPeriod) * time.Second / time.Duration(te.config.Execution.NoOfThreads)

	// Start worker goroutines
	startTime := time.Now()
	for _, task := range tasks {
		wg.Add(1)
		go te.userCreationWorker(task, resultChan, &wg)
		
		// Ramp-up delay
		if rampUpDelay > 0 {
			time.Sleep(rampUpDelay)
		}
	}
	
	// Wait for all workers to complete
	wg.Wait()
	close(resultChan)
	
	duration := time.Since(startTime)
	fmt.Printf("User creation completed in %v\n", duration)
	return nil
}

// userCreationWorker creates users for all tenants within the assigned user range
func (te *TestExecutor) userCreationWorker(task WorkerTask, resultChan chan<- TestResult, wg *sync.WaitGroup) {
	defer wg.Done()
	
	startTime := time.Now()
	fmt.Printf("Thread %d: Creating users %d-%d for all tenants\n", 
		task.ThreadID, task.UserStart, task.UserEnd)
	
	for userIndex := task.UserStart; userIndex <= task.UserEnd; userIndex++ {
		// Create this user for all tenants
		for tenantIndex := te.config.Execution.TenantStartNumber; tenantIndex < te.config.Execution.TenantStartNumber+te.config.Execution.NoOfTenants; tenantIndex++ {
			result := TestResult{
				TenantIndex: tenantIndex,
				UserIndex:   userIndex,
				ThreadID:    task.ThreadID,
			}
			
			userResp, err := task.Client.CreateUser(tenantIndex, userIndex)
			if err != nil {
				result.Success = false
				result.Error = err
				
				// Generate the username that was attempted
				username := te.config.GetTestUsername(userIndex)
				
				// Write failed user to CSV file (only if not in retry mode)
				if te.failedUsersWriter != nil {
					timestamp := time.Now().Format("2006-01-02 15:04:05")
					if csvErr := te.failedUsersWriter.WriteFailedUser(tenantIndex, username, err.Error(), timestamp); csvErr != nil {
						fmt.Printf("Thread %d: Failed to write failed user (Tenant:%d, Username:%s) to CSV: %v\n", task.ThreadID, tenantIndex, username, csvErr)
					}
				}
				
				fmt.Printf("Thread %d: Failed to create user %d for tenant %d: %v\n", 
					task.ThreadID, userIndex, tenantIndex, err)
			} else {
				result.Success = true
				result.ScimID = userResp.ID
			}
			
			resultChan <- result
		}
	}
	
	duration := time.Since(startTime)
	fmt.Printf("Thread %d: Completed users %d-%d for all tenants in %v\n", task.ThreadID, task.UserStart, task.UserEnd, duration)
}