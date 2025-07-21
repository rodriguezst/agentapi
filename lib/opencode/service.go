package opencode

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// Service manages an OpenCode server process
type Service struct {
	cmd    *exec.Cmd
	port   int
	logger *slog.Logger
	client *Client
}

// NewService creates a new OpenCode service
func NewService(logger *slog.Logger, port int) *Service {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	return &Service{
		port:   port,
		logger: logger,
		client: NewClient(baseURL),
	}
}

// Start starts the OpenCode server
func (s *Service) Start(ctx context.Context) error {
	// Check if we should skip starting the actual process (for testing)
	if os.Getenv("OPENCODE_MOCK_URL") != "" {
		s.logger.Info("Using mock OpenCode server", "url", os.Getenv("OPENCODE_MOCK_URL"))
		s.client = NewClient(os.Getenv("OPENCODE_MOCK_URL"))
		return s.client.WaitForReady(ctx, 30*time.Second)
	}
	
	// Start the opencode serve process
	s.cmd = exec.CommandContext(ctx, "opencode", "serve", "--port", strconv.Itoa(s.port), "--hostname", "127.0.0.1")
	s.cmd.Env = append(os.Environ())
	
	// Redirect stdout and stderr to help with debugging
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr
	
	s.logger.Info("Starting OpenCode server", "port", s.port)
	
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start opencode serve: %w", err)
	}
	
	// Wait for the server to be ready
	if err := s.client.WaitForReady(ctx, 30*time.Second); err != nil {
		s.Stop()
		return fmt.Errorf("opencode server failed to start: %w", err)
	}
	
	s.logger.Info("OpenCode server started successfully", "port", s.port)
	return nil
}

// Stop stops the OpenCode server
func (s *Service) Stop() error {
	// If using mock URL, no process to stop
	if os.Getenv("OPENCODE_MOCK_URL") != "" {
		s.logger.Info("Mock OpenCode server, no process to stop")
		return nil
	}
	
	if s.cmd != nil && s.cmd.Process != nil {
		s.logger.Info("Stopping OpenCode server")
		
		// Try graceful shutdown first
		if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
			s.logger.Warn("Failed to send interrupt signal", "error", err)
		}
		
		// Wait a bit for graceful shutdown
		done := make(chan error, 1)
		go func() {
			done <- s.cmd.Wait()
		}()
		
		select {
		case <-done:
			s.logger.Info("OpenCode server stopped gracefully")
		case <-time.After(5 * time.Second):
			s.logger.Warn("Force killing OpenCode server")
			if err := s.cmd.Process.Kill(); err != nil {
				s.logger.Error("Failed to kill OpenCode server", "error", err)
				return err
			}
			<-done // Wait for process to actually exit
		}
	}
	return nil
}

// Client returns the HTTP client for communicating with the OpenCode server
func (s *Service) Client() *Client {
	return s.client
}

// IsRunning checks if the OpenCode server is running
func (s *Service) IsRunning() bool {
	// If using mock URL, always return true
	if os.Getenv("OPENCODE_MOCK_URL") != "" {
		return true
	}
	
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	
	// Check if process is still alive
	err := s.cmd.Process.Signal(os.Signal(syscall.Signal(0)))
	return err == nil
}