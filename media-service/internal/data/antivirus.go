package data

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/media-service/internal/biz"
)

type clamAVScanner struct {
	host    string
	enabled bool
}

type AntivirusConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ClamAVHost  string `yaml:"clamav_host"`
}

func NewClamAVScanner(config AntivirusConfig) biz.AntivirusScanner {
	return &clamAVScanner{
		host:    config.ClamAVHost,
		enabled: config.Enabled,
	}
}

func (s *clamAVScanner) ScanFile(ctx context.Context, objectKey string) (bool, error) {
	if !s.enabled {
		// If antivirus is disabled, always return clean
		return true, nil
	}

	// This is a simplified mock implementation
	// In a real implementation, you would:
	// 1. Download the file from storage
	// 2. Send it to ClamAV daemon for scanning
	// 3. Parse the response
	
	// For now, we'll just check if ClamAV is reachable
	conn, err := net.DialTimeout("tcp", s.host, 5*time.Second)
	if err != nil {
		// If ClamAV is not reachable, log error and assume clean
		// In production, you might want to fail safe or retry
		return true, fmt.Errorf("clamav not reachable: %v", err)
	}
	defer conn.Close()

	// Mock scan - in reality you'd send the file content
	// For demo purposes, we'll assume all files are clean
	return true, nil
}

// MockAntivirusScanner is a simple mock for testing
type mockAntivirusScanner struct{}

func NewMockAntivirusScanner() biz.AntivirusScanner {
	return &mockAntivirusScanner{}
}

func (s *mockAntivirusScanner) ScanFile(ctx context.Context, objectKey string) (bool, error) {
	// Mock implementation - always returns clean
	// You could add logic here to simulate infected files for testing
	return true, nil
}
