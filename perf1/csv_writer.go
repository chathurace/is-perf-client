package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sync"
)

// CSVWriter handles writing SCIM IDs to CSV file
type CSVWriter struct {
	filename string
	file     *os.File
	writer   *csv.Writer
	mutex    sync.Mutex
}

// NewCSVWriter creates a new CSV writer for SCIM IDs
func NewCSVWriter(filename string) (*CSVWriter, error) {
	// Delete file if it exists
	if _, err := os.Stat(filename); err == nil {
		if err := os.Remove(filename); err != nil {
			return nil, fmt.Errorf("failed to remove existing CSV file: %v", err)
		}
	}
	
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSV file: %v", err)
	}
	
	writer := csv.NewWriter(file)
	csvWriter := &CSVWriter{
		filename: filename,
		file:     file,
		writer:   writer,
	}
	
	// Write header
	if err := csvWriter.writeHeader(); err != nil {
		file.Close()
		return nil, err
	}
	
	return csvWriter, nil
}

// writeHeader writes the CSV header
func (c *CSVWriter) writeHeader() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	return c.writer.Write([]string{"scim_id"})
}

// WriteScimID writes a SCIM ID to the CSV file
func (c *CSVWriter) WriteScimID(scimID string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if err := c.writer.Write([]string{scimID}); err != nil {
		return fmt.Errorf("failed to write SCIM ID to CSV: %v", err)
	}
	
	// Flush to ensure data is written
	c.writer.Flush()
	return c.writer.Error()
}

// Close closes the CSV writer and file
func (c *CSVWriter) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.writer.Flush()
	if err := c.writer.Error(); err != nil {
		c.file.Close()
		return fmt.Errorf("CSV writer error: %v", err)
	}
	
	return c.file.Close()
}

// FailedUsersCSVWriter handles writing failed user creation attempts to CSV file
type FailedUsersCSVWriter struct {
	filename string
	file     *os.File
	writer   *csv.Writer
	mutex    sync.Mutex
}

// NewFailedUsersCSVWriter creates a new CSV writer for failed users
func NewFailedUsersCSVWriter(filename string) (*FailedUsersCSVWriter, error) {
	// Delete file if it exists
	if _, err := os.Stat(filename); err == nil {
		if err := os.Remove(filename); err != nil {
			return nil, fmt.Errorf("failed to remove existing failed users CSV file: %v", err)
		}
	}
	
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create failed users CSV file: %v", err)
	}
	
	writer := csv.NewWriter(file)
	
	// Write header
	if err := writer.Write([]string{"TenantID", "Username", "Error", "Timestamp"}); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to write CSV header: %v", err)
	}
	writer.Flush()
	
	return &FailedUsersCSVWriter{
		filename: filename,
		file:     file,
		writer:   writer,
	}, nil
}

// NewFailedUsersCSVWriterAppend creates a new CSV writer for failed users in append mode
func NewFailedUsersCSVWriterAppend(filename string) (*FailedUsersCSVWriter, error) {
	// Open file in append mode, create if it doesn't exist
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open/create failed users CSV file: %v", err)
	}
	
	writer := csv.NewWriter(file)
	
	// Check if file is empty and write header if needed
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to get file stats: %v", err)
	}
	
	if stat.Size() == 0 {
		// File is empty, write header
		if err := writer.Write([]string{"TenantID", "Username", "Error", "Timestamp"}); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to write CSV header: %v", err)
		}
		writer.Flush()
	}
	
	return &FailedUsersCSVWriter{
		filename: filename,
		file:     file,
		writer:   writer,
	}, nil
}

// WriteFailedUser writes a failed user creation attempt to the CSV file
func (fw *FailedUsersCSVWriter) WriteFailedUser(tenantID int, username, errorMsg, timestamp string) error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()
	
	record := []string{
		fmt.Sprintf("%d", tenantID),
		username,
		errorMsg,
		timestamp,
	}
	
	if err := fw.writer.Write(record); err != nil {
		return fmt.Errorf("failed to write failed user record: %v", err)
	}
	
	fw.writer.Flush()
	return fw.writer.Error()
}

// Close closes the failed users CSV writer
func (fw *FailedUsersCSVWriter) Close() error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()
	
	if fw.writer != nil {
		fw.writer.Flush()
	}
	
	if fw.file != nil {
		return fw.file.Close()
	}
	
	return nil
}
