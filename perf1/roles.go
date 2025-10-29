package main

import (
	"fmt"
	"sync"
)

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