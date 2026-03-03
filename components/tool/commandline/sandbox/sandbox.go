/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/commandline"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
)

// Config configures the sandbox environment
type Config struct {
	VolumeBindings map[string]string
	Image          string
	HostName       string
	WorkDir        string
	Env            []string
	MemoryLimit    int64   // Memory limit in bytes
	CPULimit       float64 // CPU limit in cores
	NetworkEnabled bool
	Timeout        time.Duration // Command execution timeout in seconds
}

// DockerSandbox provides a containerized execution environment
type DockerSandbox struct {
	config      Config
	client      *client.Client
	containerID string
}

const (
	defaultImage       = "python:3.9-slim"
	defaultWorkDir     = "/workspace"
	defaultMemoryLimit = 512 * 1024 * 1024 // 512M
	defaultCPULimit    = 1.0
	defaultTimeout     = time.Second * 30
	defaultHostName    = "sandbox"
)

// NewDockerSandbox creates a new Docker sandbox with the given configuration
func NewDockerSandbox(ctx context.Context, config *Config) (*DockerSandbox, error) {
	if config == nil {
		config = &Config{}
	} else {
		nConfig := *config
		config = &nConfig
	}

	if len(config.Image) == 0 {
		config.Image = defaultImage
	}
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	if config.WorkDir == "" {
		config.WorkDir = defaultWorkDir
	}
	if config.MemoryLimit == 0 {
		config.MemoryLimit = defaultMemoryLimit
	}
	if config.CPULimit == 0 {
		config.CPULimit = defaultCPULimit
	}
	if config.VolumeBindings == nil {
		config.VolumeBindings = make(map[string]string)
	}
	if len(config.HostName) == 0 {
		config.HostName = defaultHostName
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerSandbox{
		config: *config,
		client: cli,
	}, nil
}

// Create creates and starts the sandbox container
func (s *DockerSandbox) Create(ctx context.Context) error {
	// Prepare container configuration
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:    s.config.MemoryLimit,
			CPUPeriod: 100000,
			CPUQuota:  int64(100000 * s.config.CPULimit),
		},
		NetworkMode: "none",
	}

	if s.config.NetworkEnabled {
		hostConfig.NetworkMode = "bridge"
	}

	// Prepare volume bindings
	binds, err := s.prepareVolumeBindings()
	if err != nil {
		return fmt.Errorf("failed to prepare volume bindings: %w", err)
	}
	hostConfig.Binds = binds

	// Generate unique container name
	containerName := fmt.Sprintf("sandbox_%s", uuid.New().String()[:8])

	// Create container
	resp, err := s.client.ContainerCreate(
		ctx,
		&container.Config{
			Image:      s.config.Image,
			Cmd:        []string{},
			Hostname:   s.config.HostName,
			WorkingDir: s.config.WorkDir,
			Tty:        true,
			Env:        s.config.Env,
		},
		hostConfig,
		nil,
		nil,
		containerName,
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	s.containerID = resp.ID

	// Start container
	if err := s.client.ContainerStart(ctx, s.containerID, container.StartOptions{}); err != nil {
		// Clean up resources
		s.Cleanup(ctx)
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

// RunCommand executes a command with explicit arguments (no shell involved)
func (s *DockerSandbox) RunCommand(ctx context.Context, cmd []string) (*commandline.CommandOutput, error) {
	if s.containerID == "" {
		return nil, fmt.Errorf("sandbox not initialized")
	}

	timeout := s.config.Timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   s.config.WorkDir,
	}

	execID, err := s.client.ContainerExecCreate(ctx, s.containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec instance: %w", err)
	}

	resp, err := s.client.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{
		Detach:      false,
		Tty:         false,
		ConsoleSize: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec instance: %w", err)
	}
	defer resp.Close()

	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)
	go func() {
		_, err := stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return nil, fmt.Errorf("failed to read output: %w", err)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("command timed out after %v", timeout)
	}

	inspectResp, err := s.client.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect exec: %w", err)
	}

	return &commandline.CommandOutput{Stdout: outBuf.String(), Stderr: errBuf.String(), ExitCode: inspectResp.ExitCode}, nil
}

// ReadFile reads a file from the container
func (s *DockerSandbox) ReadFile(ctx context.Context, path string) (string, error) {
	if s.containerID == "" {
		return "", fmt.Errorf("sandbox not initialized")
	}

	// Resolve path
	resolvedPath, err := s.safeResolvePath(path)
	if err != nil {
		return "", err
	}

	// Get file content
	reader, _, err := s.client.CopyFromContainer(ctx, s.containerID, resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to copy from container: %w", err)
	}
	defer reader.Close()

	// Read file content from tar stream
	content, err := readFromTar(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	return string(content), nil
}

// WriteFile writes content to a file in the container
func (s *DockerSandbox) WriteFile(ctx context.Context, path string, content string) error {
	if s.containerID == "" {
		return fmt.Errorf("sandbox not initialized")
	}

	// Resolve path
	resolvedPath, err := s.safeResolvePath(path)
	if err != nil {
		return err
	}

	// Create parent directory
	parentDir := filepath.Dir(resolvedPath)
	if parentDir != "" && parentDir != "/" {
		_, err := s.RunCommand(ctx, []string{"mkdir", "-p", parentDir})
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Create tar stream
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	// Add file to tar
	hdr := &tar.Header{
		Name: filepath.Base(path),
		Mode: 0644,
		Size: int64(len(content)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	if _, err := tw.Write([]byte(content)); err != nil {
		return fmt.Errorf("failed to write tar content: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Upload to container
	destDir := filepath.Dir(resolvedPath)
	if destDir == "" {
		destDir = "/"
	}

	err = s.client.CopyToContainer(ctx, s.containerID, destDir, bytes.NewReader(tarBuf.Bytes()), container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
		CopyUIDGID:                false,
	})
	if err != nil {
		return fmt.Errorf("failed to copy to container: %w", err)
	}

	return nil
}

// IsDirectory checks if a path in the container is a directory
func (s *DockerSandbox) IsDirectory(ctx context.Context, path string) (bool, error) {
	if s.containerID == "" {
		return false, fmt.Errorf("sandbox not initialized")
	}

	// Resolve path
	resolvedPath, err := s.safeResolvePath(path)
	if err != nil {
		return false, err
	}

	// Use stat command to check path type
	cmd := []string{"test", "-d", resolvedPath}
	cmdOutput, err := s.RunCommand(ctx, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check path existence: %w", err)
	}

	return cmdOutput.ExitCode == 0, nil
}

// Exists checks if a path exists in the container
func (s *DockerSandbox) Exists(ctx context.Context, path string) (bool, error) {
	if s.containerID == "" {
		return false, fmt.Errorf("sandbox not initialized")
	}

	// Resolve path
	resolvedPath, err := s.safeResolvePath(path)
	if err != nil {
		return false, err
	}

	// Use stat command to check if path exists
	cmd := []string{"test", "-e", resolvedPath}
	cmdOutput, err := s.RunCommand(ctx, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check path existence: %w", err)
	}

	return cmdOutput.ExitCode == 0, nil
}

// Cleanup cleans up sandbox resources
func (s *DockerSandbox) Cleanup(ctx context.Context) {
	var errors []string

	if s.containerID != "" {
		// Stop container
		timeout := 5 // seconds
		err := s.client.ContainerStop(ctx, s.containerID, container.StopOptions{
			Signal:  "",
			Timeout: &timeout,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("error stopping container: %v", err))
		}

		// Remove container
		err = s.client.ContainerRemove(ctx, s.containerID, container.RemoveOptions{
			RemoveVolumes: false,
			RemoveLinks:   false,
			Force:         true,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("error removing container: %v", err))
		}

		s.containerID = ""
	}

	if len(errors) > 0 {
		log.Printf("Warning: errors occurred during cleanup: %s\n", strings.Join(errors, ", "))
	}
}

// prepareVolumeBindings prepares volume bindings
func (s *DockerSandbox) prepareVolumeBindings() ([]string, error) {
	binds := []string{}

	// If the user already provided a volume binding that maps to the
	// configured WorkDir, skip creating an automatic temp host directory
	// mapping to avoid duplicate mount points.
	skipAuto := false
	for _, containerPath := range s.config.VolumeBindings {
		if containerPath == s.config.WorkDir {
			skipAuto = true
			break
		}
	}

	// Create and add working directory mapping unless user already bound it
	if !skipAuto {
		workDir, err := s.ensureHostDir(s.config.WorkDir)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure host directory exists: %w", err)
		}

		binds = append(binds, fmt.Sprintf("%s:%s:rw", workDir, s.config.WorkDir))
	}

	// Add custom volume bindings
	for hostPath, containerPath := range s.config.VolumeBindings {
		binds = append(binds, fmt.Sprintf("%s:%s:rw", hostPath, containerPath))
	}

	return binds, nil
}

// ensureHostDir ensures host directory exists
func (s *DockerSandbox) ensureHostDir(path string) (string, error) {
	// Create temporary directory
	tempDir := os.TempDir()
	randomID := uuid.New().String()[:8]
	hostPath := filepath.Join(tempDir, fmt.Sprintf("sandbox_%s_%s", filepath.Base(path), randomID))

	if err := os.MkdirAll(hostPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create host directory: %w", err)
	}

	return hostPath, nil
}

// safeResolvePath safely resolves path
func (s *DockerSandbox) safeResolvePath(path string) (string, error) {
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path contains potentially unsafe pattern")
	}

	var resolved string
	if filepath.IsAbs(path) {
		resolved = path
	} else {
		resolved = filepath.Join(s.config.WorkDir, path)
	}

	return resolved, nil
}

// readFromTar reads file content from tar stream
func readFromTar(reader io.Reader) ([]byte, error) {
	tr := tar.NewReader(reader)

	// Read first file
	_, err := tr.Next()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("empty tar archive")
		}
		return nil, err
	}

	// Read file content
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, tr); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
