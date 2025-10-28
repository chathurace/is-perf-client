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

// TestExecutor handles the execution of the SCIM2 test
type TestExecutor struct {
	config            *Config
	// client            *HTTPClient
	csvWriter         *CSVWriter
	failedUsersWriter *FailedUsersCSVWriter
	stats             *TestStats
}

// NewTestExecutor creates a new test executor
func NewTestExecutor(config *Config, retryMode bool) (*TestExecutor, error) {
	// client := NewHTTPClient(config)
	
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
		// client:            client,
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

// ExecuteRoleCreation creates roles for all tenants concurrently
func (te *TestExecutor) ExecuteRoleCreation() error {
	fmt.Println("Starting role creation phase...")
	
	totalTenants := te.config.Execution.NoOfTenants
	threads := te.config.Execution.NoOfThreads
	
	// Calculate tenants per thread
	tenantsPerThread := totalTenants / threads
	remainingTenants := totalTenants % threads
	
	// Create wait group for synchronization
	var wg sync.WaitGroup
	tenantStart := te.config.Execution.TenantStartNumber
	
	// Start worker goroutines for role creation
	for threadID := 0; threadID < threads; threadID++ {
		threadTenants := tenantsPerThread
		if threadID < remainingTenants {
			threadTenants++ // Distribute remaining tenants to first few threads
		}
		
		tenantEnd := tenantStart + threadTenants - 1
		
		if threadTenants > 0 {
			// Create a separate HTTP client for this thread
			threadClient := NewHTTPClient(te.config)
			
			wg.Add(1)
			go te.roleCreationWorker(threadID, tenantStart, tenantEnd, threadClient, &wg)
		}
		
		tenantStart = tenantEnd + 1
	}
	
	// Wait for all workers to complete
	wg.Wait()
	
	fmt.Println("Role creation phase completed.")
	return nil
}

// roleCreationWorker creates roles for a specific range of tenants
func (te *TestExecutor) roleCreationWorker(threadID, tenantStart, tenantEnd int, client *HTTPClient, wg *sync.WaitGroup) {
	defer wg.Done()
	
	fmt.Printf("Thread %d: Creating roles for tenants %d-%d\n", threadID, tenantStart, tenantEnd)
	
	for tenantIndex := tenantStart; tenantIndex <= tenantEnd; tenantIndex++ {
		fmt.Printf("Thread %d: Creating role for tenant %d...\n", threadID, tenantIndex)
		
		err := client.CreateRole(tenantIndex)
		te.stats.IncrementRole(err == nil)
		
		if err != nil {
			fmt.Printf("Thread %d: Failed to create role for tenant %d: %v\n", threadID, tenantIndex, err)
			// Continue with other tenants even if one fails
		} else {
			// fmt.Printf("Thread %d: Role created successfully for tenant %d\n", threadID, tenantIndex)
		}
	}
	
	fmt.Printf("Thread %d: Completed role creation for tenants %d-%d\n", threadID, tenantStart, tenantEnd)
}

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
	
	fmt.Println("User creation phase completed.")
	return nil
}

// userCreationWorker creates users for all tenants within the assigned user range
func (te *TestExecutor) userCreationWorker(task WorkerTask, resultChan chan<- TestResult, wg *sync.WaitGroup) {
	defer wg.Done()
	
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
	
	fmt.Printf("Thread %d: Completed users %d-%d for all tenants\n", task.ThreadID, task.UserStart, task.UserEnd)
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
