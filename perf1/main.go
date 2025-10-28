package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {
	var configPath string
	var generateConfig bool
	var retryFailed bool
	
	flag.StringVar(&configPath, "config", "", "Path to configuration file (JSON)")
	flag.BoolVar(&generateConfig, "generate-config", false, "Generate default configuration file")
	flag.BoolVar(&retryFailed, "retry-failed", false, "Retry only failed users from failedUsers.csv")
	
	// Parse flags first to handle help and generate-config
	flag.Parse()
	
	// Handle generate config option
	if generateConfig {
		if configPath == "" {
			configPath = "config.json"
		}
		
		config := DefaultConfig()
		if err := config.SaveConfig(configPath); err != nil {
			log.Fatalf("Failed to generate config file: %v", err)
		}
		
		fmt.Printf("Default configuration saved to: %s\n", configPath)
		fmt.Println("You can modify this file and run with -config flag")
		return
	}
	
	// Load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	// Print configuration summary
	fmt.Println("=== SCIM2 Test Configuration ===")
	fmt.Printf("Server: %s\n", config.GetServerURL())
	fmt.Printf("Username: %s\n", config.Server.Username)
	fmt.Printf("Tenant Prefix: %s\n", config.Test.TenantPrefix)
	fmt.Printf("Role Name: %s\n", config.Test.RoleName)
	fmt.Printf("Username Prefix: %s\n", config.Test.UsernamePrefix)
	fmt.Printf("Threads: %d\n", config.Execution.NoOfThreads)
	fmt.Printf("Users: %d\n", config.Execution.NoOfUsers)
	fmt.Printf("Tenants: %d\n", config.Execution.NoOfTenants)
	fmt.Printf("Ramp-up Period: %d seconds\n", config.Execution.RampUpPeriod)
	fmt.Printf("CSV Output: %s\n", config.Execution.ScimIdCsvPath)
	fmt.Println("===============================")
	fmt.Println()
	
	// Create and execute test
	executor, err := NewTestExecutor(config, retryFailed)
	if err != nil {
		log.Fatalf("Failed to create test executor: %v", err)
	}
	defer executor.Close()

	// Execute the test
	if retryFailed {
		if err := executor.ExecuteRetryFailed(); err != nil {
			log.Fatalf("Retry failed users execution failed: %v", err)
		}
	} else {
		if err := executor.Execute(); err != nil {
			log.Fatalf("Test execution failed: %v", err)
		}
	}

	fmt.Println("Test execution completed successfully!")
}
