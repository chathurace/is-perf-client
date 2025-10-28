# go-perf

A Go implementation of SCIM2 user creation performance testing, equivalent to the JMeter test plan.

## Features

- **SCIM2 User Creation**: Creates users via WSO2 Identity Server SCIM2 API
- **Role Management**: Creates roles via SOAP API before user creation  
- **Multi-tenant Support**: Supports creating users across multiple tenants
- **Concurrent Execution**: Configurable number of threads for parallel user creation
- **CSV Output**: Exports created user SCIM IDs to CSV file
- **Flexible Configuration**: JSON-based configuration with command-line overrides

## Getting Started

### Prerequisites
- Go 1.21 or later
- WSO2 Identity Server running and accessible

### Building the project
```bash
go build -o go-perf .
```

### Configuration

#### Generate default configuration
```bash
./go-perf -generate-config
```

This creates a `config.json` file with default values that you can modify.

#### Using configuration file
```bash
./go-perf -config config.json
```

#### Command line parameters
You can override any config value via command line flags:

```bash
./go-perf -host localhost -port 9443 -concurrency 5 -userCount 1000 -noOfTenants 10
```

### Configuration Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `host` | Identity Server hostname | localhost |
| `port` | Identity Server port | 9443 |
| `username` | Admin username | admin@wso2.com |
| `password` | Admin password | tpass |
| `usernamePrefix` | Prefix for test usernames | isTestUser_ |
| `userPassword` | Password for test users | Password_1 |
| `userRole` | Role name for test users | isTestUserRole |
| `tenantPrefix` | Tenant prefix | tenant |
| `concurrency` | Number of concurrent threads | 3 |
| `userCount` | Total users to create | 100 |
| `noOfTenants` | Number of tenants | 5 |
| `rampUpPeriod` | Ramp up period in seconds | 10 |
| `scimIdCsvPath` | Output CSV file path | scimIDs.csv |
| `userStartNumber` | Starting user number | 1 |
| `tenantStartNumber` | Starting tenant number | 1 |

### Example Usage

#### Basic usage with defaults
```bash
./go-perf
```

#### Create 500 users across 10 tenants with 5 threads
```bash
./go-perf -userCount 500 -noOfTenants 10 -concurrency 5
```

#### Use custom server
```bash
./go-perf -host my-is-server.com -port 9443 -username admin@carbon.super -password admin
```

## Test Flow

The application follows the same logic as the original JMeter test:

1. **Role Creation Phase**: Creates a role in each tenant using SOAP API
2. **User Creation Phase**: Creates users in parallel across all tenants using SCIM2 REST API
3. **Result Collection**: Collects SCIM IDs and writes them to CSV file
4. **Statistics**: Reports success/failure rates and execution time

## Output

- **Console**: Real-time progress and statistics
- **CSV File**: SCIM IDs of successfully created users
- **Statistics**: Final summary of success/failure rates

## Project Structure

```
go-perf/
├── main.go          # Main entry point
├── config.go        # Configuration handling
├── http_client.go   # HTTP client for SOAP/REST APIs
├── executor.go      # Test execution logic
├── csv_writer.go    # CSV file handling
├── config.json      # Sample configuration
└── README.md        # This file
```
