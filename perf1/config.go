package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

// Config represents the configuration for the SCIM2 test
type Config struct {
	// Server Variables
	Server ServerConfig `json:"server"`
	
	// Test Variables
	Test TestConfig `json:"test"`
	
	// User Defined Variables
	Execution ExecutionConfig `json:"execution"`
}

// ServerConfig holds server connection details
type ServerConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// TestConfig holds test-specific parameters
type TestConfig struct {
	UsernamePrefix string `json:"usernamePrefix"`
	UserPassword   string `json:"userPassword"`
	RoleName       string `json:"roleName"`
	TenantPrefix   string `json:"tenantPrefix"`
}

// ExecutionConfig holds execution parameters
type ExecutionConfig struct {
	NoOfThreads       int    `json:"noOfThreads"`
	NoOfUsers         int    `json:"noOfUsers"`
	LoopCount         int    `json:"loopCount"`
	RampUpPeriod      int    `json:"rampUpPeriod"`
	ScimIdCsvPath     string `json:"scimIdCsvPath"`
	FailedUsersCsvPath string `json:"failedUsersCsvPath"`
	NoOfTenants       int    `json:"noOfTenants"`
	UserStartNumber   int    `json:"userStartNumber"`
	TenantStartNumber int    `json:"tenantStartNumber"`
}

// DefaultConfig returns a configuration with default values matching the JMX file
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:     "localhost",
			Port:     9443,
			Username: "admin@wso2.com",
			Password: "tpass",
		},
		Test: TestConfig{
			UsernamePrefix: "isTestUser_",
			UserPassword:   "Password_1",
			RoleName:       "isTestUserRole",
			TenantPrefix:   "tenant",
		},
		Execution: ExecutionConfig{
			NoOfThreads:        1,
			NoOfUsers:          1000,
			LoopCount:          1000,
			RampUpPeriod:       10,
			ScimIdCsvPath:      "scimIDs.csv",
			FailedUsersCsvPath: "failedUsers.csv",
			NoOfTenants:        5,
			UserStartNumber:    1,
			TenantStartNumber:  1,
		},
	}
}

// LoadConfig loads configuration from file or returns default config
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()
	
	if configPath != "" {
		file, err := os.Open(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open config file: %v", err)
		}
		defer file.Close()
		
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %v", err)
		}
		
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %v", err)
		}
	}
	
	// Override with command line flags if provided
	parseFlags(config)
	
	return config, nil
}

// parseFlags parses command line flags and overrides config values
func parseFlags(config *Config) {
	flag.StringVar(&config.Server.Host, "host", config.Server.Host, "Server host")
	flag.IntVar(&config.Server.Port, "port", config.Server.Port, "Server port")
	flag.StringVar(&config.Server.Username, "username", config.Server.Username, "Admin username")
	flag.StringVar(&config.Server.Password, "password", config.Server.Password, "Admin password")
	
	flag.StringVar(&config.Test.UsernamePrefix, "usernamePrefix", config.Test.UsernamePrefix, "Username prefix for test users")
	flag.StringVar(&config.Test.UserPassword, "userPassword", config.Test.UserPassword, "Password for test users")
	flag.StringVar(&config.Test.RoleName, "userRole", config.Test.RoleName, "Role name for test users")
	flag.StringVar(&config.Test.TenantPrefix, "tenantPrefix", config.Test.TenantPrefix, "Tenant prefix")
	
	flag.IntVar(&config.Execution.NoOfThreads, "concurrency", config.Execution.NoOfThreads, "Number of concurrent threads")
	flag.IntVar(&config.Execution.NoOfUsers, "userCount", config.Execution.NoOfUsers, "Total number of users to create")
	flag.IntVar(&config.Execution.LoopCount, "loopCount", config.Execution.LoopCount, "Loop count")
	flag.IntVar(&config.Execution.RampUpPeriod, "rampUpPeriod", config.Execution.RampUpPeriod, "Ramp up period in seconds")
	flag.StringVar(&config.Execution.ScimIdCsvPath, "scimIdCsvPath", config.Execution.ScimIdCsvPath, "Path to SCIM ID CSV file")
	flag.IntVar(&config.Execution.NoOfTenants, "noOfTenants", config.Execution.NoOfTenants, "Number of tenants")
	flag.IntVar(&config.Execution.UserStartNumber, "userStartNumber", config.Execution.UserStartNumber, "Starting user number")
	flag.IntVar(&config.Execution.TenantStartNumber, "tenantStartNumber", config.Execution.TenantStartNumber, "Starting tenant number")
	
	flag.Parse()
}

// SaveConfig saves the current configuration to a file
func (c *Config) SaveConfig(configPath string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	
	return nil
}

// GetTenantUsername returns the tenant-specific username
func (c *Config) GetTenantUsername(tenantIndex int) string {
	// Format: admin@wso2.com@aorg_11.com (base@tenantPrefix+tenantIndex+.com)
	return fmt.Sprintf("%s@%s%d.com", c.Server.Username, c.Test.TenantPrefix, tenantIndex)
}

// GetTestUsername returns the test user username
func (c *Config) GetTestUsername(userIndex int) string {
	return fmt.Sprintf("%s%d", c.Test.UsernamePrefix, userIndex)
}

// GetServerURL returns the full server URL
func (c *Config) GetServerURL() string {
	return fmt.Sprintf("https://%s:%d", c.Server.Host, c.Server.Port)
}
