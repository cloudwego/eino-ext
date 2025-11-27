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
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/stretchr/testify/assert"
)

func TestNewDockerSandbox(t *testing.T) {
	// 测试默认配置
	ctx := context.Background()
	sandbox, err := NewDockerSandbox(ctx, nil)
	assert.NoError(t, err)
	assert.NotNil(t, sandbox)
	assert.Equal(t, "python:3.9-slim", sandbox.config.Image)
	assert.Equal(t, "/workspace", sandbox.config.WorkDir)
	assert.Equal(t, int64(512*1024*1024), sandbox.config.MemoryLimit)
	assert.Equal(t, 1.0, sandbox.config.CPULimit)
	assert.False(t, sandbox.config.NetworkEnabled)
	assert.Equal(t, time.Second*30, sandbox.config.Timeout)

	// 测试自定义配置
	customConfig := &Config{
		Image:          "node:14",
		WorkDir:        "/app",
		MemoryLimit:    1024 * 1024 * 1024,
		CPULimit:       2.0,
		NetworkEnabled: true,
		Timeout:        60 * time.Second,
	}
	sandbox, err = NewDockerSandbox(ctx, customConfig)
	assert.NoError(t, err)
	assert.NotNil(t, sandbox)
	assert.Equal(t, "node:14", sandbox.config.Image)
	assert.Equal(t, "/app", sandbox.config.WorkDir)
	assert.Equal(t, int64(1024*1024*1024), sandbox.config.MemoryLimit)
	assert.Equal(t, 2.0, sandbox.config.CPULimit)
	assert.True(t, sandbox.config.NetworkEnabled)
	assert.Equal(t, 60*time.Second, sandbox.config.Timeout)
}

func TestDockerSandbox_Create(t *testing.T) {
	ctx := context.Background()

	defer mockey.Mock((*client.Client).ContainerCreate).Return(container.CreateResponse{ID: "test_container_id"}, nil).Build().UnPatch()
	defer mockey.Mock((*client.Client).ContainerStart).To(func(ctx context.Context, containerID string, options container.StartOptions) error {
		assert.Equal(t, "test_container_id", containerID)
		return nil
	}).Build().UnPatch()

	// 创建沙箱实例
	sandbox := &DockerSandbox{
		config: Config{
			Image:          "python:3.9-slim",
			WorkDir:        "/workspace",
			MemoryLimit:    512 * 1024 * 1024,
			CPULimit:       1.0,
			NetworkEnabled: false,
			Timeout:        30 * time.Second,
		},
		client: &client.Client{},
	}

	// 测试创建容器
	err := sandbox.Create(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "test_container_id", sandbox.containerID)
}

func TestDockerSandbox_prepareVolumeBindings(t *testing.T) {
	sandbox := &DockerSandbox{
		config: Config{
			WorkDir: "/workspace",
			VolumeBindings: map[string]string{
				"/host/custom": "/workspace",
			},
		},
	}

	binds, err := sandbox.prepareVolumeBindings()
	assert.NoError(t, err)
	assert.Len(t, binds, 1)
	assert.Equal(t, "/host/custom:/workspace:rw", binds[0])
}

func TestDockerSandbox_RunCommand(t *testing.T) {
	ctx := context.Background()

	defer mockey.Mock((*client.Client).ContainerExecCreate).Return(container.ExecCreateResponse{ID: "test_exec_id"}, nil).Build().UnPatch()
	buf := &bytes.Buffer{}
	sw := stdcopy.NewStdWriter(buf, stdcopy.Stdout)
	sw.Write([]byte("success"))
	defer mockey.Mock((*client.Client).ContainerExecAttach).To(func(ctx context.Context, execID string, config container.ExecAttachOptions) (types.HijackedResponse, error) {
		assert.Equal(t, "test_exec_id", execID)
		return types.HijackedResponse{
			Conn:   &myConn{},
			Reader: bufio.NewReader(buf),
		}, nil
	}).Build().UnPatch()
	defer mockey.Mock((*client.Client).ContainerExecInspect).To(func(ctx context.Context, execID string) (container.ExecInspect, error) {
		assert.Equal(t, "test_exec_id", execID)
		return container.ExecInspect{
			ExitCode: 0,
		}, nil
	}).Build().UnPatch()

	// 创建沙箱实例
	sandbox := &DockerSandbox{
		config: Config{
			Timeout: 30 * time.Second,
		},
		client:      &client.Client{},
		containerID: "test_container_id",
	}

	// 测试执行命令
	output, err := sandbox.RunCommand(ctx, []string{"echo", "success"})
	assert.NoError(t, err)
	assert.Equal(t, "success", output.Stdout)
}

func TestDockerSandbox_IsDirectory(t *testing.T) {
	ctx := context.Background()

	defer mockey.Mock((*client.Client).ContainerExecCreate).Return(container.ExecCreateResponse{ID: "test_exec_id"}, nil).Build().UnPatch()
	buf := &bytes.Buffer{}
	sw := stdcopy.NewStdWriter(buf, stdcopy.Stdout)
	sw.Write([]byte("true"))
	defer mockey.Mock((*client.Client).ContainerExecAttach).To(func(ctx context.Context, execID string, config container.ExecAttachOptions) (types.HijackedResponse, error) {
		assert.Equal(t, "test_exec_id", execID)
		return types.HijackedResponse{
			Conn:   &myConn{},
			Reader: bufio.NewReader(buf),
		}, nil
	}).Build().UnPatch()
	defer mockey.Mock((*client.Client).ContainerExecInspect).To(func(ctx context.Context, execID string) (container.ExecInspect, error) {
		assert.Equal(t, "test_exec_id", execID)
		return container.ExecInspect{
			ExitCode: 0,
		}, nil
	}).Build().UnPatch()

	// 创建沙箱实例
	sandbox := &DockerSandbox{
		config: Config{
			Timeout: 30 * time.Second,
		},
		client:      &client.Client{},
		containerID: "test_container_id",
	}

	// 测试执行命令
	is, err := sandbox.IsDirectory(ctx, "echo 'hello world'")
	assert.NoError(t, err)
	assert.True(t, is)
}

func TestDockerSandbox_Exists(t *testing.T) {
	ctx := context.Background()

	defer mockey.Mock((*client.Client).ContainerExecCreate).Return(container.ExecCreateResponse{ID: "test_exec_id"}, nil).Build().UnPatch()
	buf := &bytes.Buffer{}
	sw := stdcopy.NewStdWriter(buf, stdcopy.Stdout)
	sw.Write([]byte("true"))
	defer mockey.Mock((*client.Client).ContainerExecAttach).To(func(ctx context.Context, execID string, config container.ExecAttachOptions) (types.HijackedResponse, error) {
		assert.Equal(t, "test_exec_id", execID)
		return types.HijackedResponse{
			Conn:   &myConn{},
			Reader: bufio.NewReader(buf),
		}, nil
	}).Build().UnPatch()
	defer mockey.Mock((*client.Client).ContainerExecInspect).To(func(ctx context.Context, execID string) (container.ExecInspect, error) {
		assert.Equal(t, "test_exec_id", execID)
		return container.ExecInspect{
			ExitCode: 0,
		}, nil
	}).Build().UnPatch()

	// 创建沙箱实例
	sandbox := &DockerSandbox{
		config: Config{
			Timeout: 30 * time.Second,
		},
		client:      &client.Client{},
		containerID: "test_container_id",
	}

	// 测试执行命令
	existed, err := sandbox.Exists(ctx, "echo 'hello world'")
	assert.NoError(t, err)
	assert.True(t, existed)
}

func TestDockerSandbox_WriteFile(t *testing.T) {
	ctx := context.Background()

	defer mockey.Mock((*client.Client).ContainerExecCreate).Return(container.ExecCreateResponse{ID: "test_exec_id"}, nil).Build().UnPatch()
	defer mockey.Mock((*client.Client).ContainerExecAttach).To(func(ctx context.Context, execID string, config container.ExecAttachOptions) (types.HijackedResponse, error) {
		assert.Equal(t, "test_exec_id", execID)
		return types.HijackedResponse{
			Conn:   &myConn{},
			Reader: bufio.NewReader(strings.NewReader("success")),
		}, nil
	}).Build().UnPatch()
	defer mockey.Mock((*client.Client).ContainerExecInspect).To(func(ctx context.Context, execID string) (container.ExecInspect, error) {
		assert.Equal(t, "test_exec_id", execID)
		return container.ExecInspect{
			ExitCode: 0,
		}, nil
	}).Build().UnPatch()
	defer mockey.Mock((*client.Client).CopyToContainer).Return(nil).Build().UnPatch()

	// 创建沙箱实例
	sandbox := &DockerSandbox{
		config: Config{
			Timeout: 30 * time.Second,
		},
		client:      &client.Client{},
		containerID: "test_container_id",
	}

	// 测试执行命令
	err := sandbox.WriteFile(ctx, "./test", "echo 'hello world'")
	assert.NoError(t, err)
}

func TestDockerSandbox_ReadFile(t *testing.T) {
	ctx := context.Background()

	defer mockey.Mock((*client.Client).ContainerExecCreate).Return(container.ExecCreateResponse{ID: "test_exec_id"}, nil).Build().UnPatch()
	defer mockey.Mock((*client.Client).ContainerExecAttach).To(func(ctx context.Context, execID string, config container.ExecAttachOptions) (types.HijackedResponse, error) {
		assert.Equal(t, "test_exec_id", execID)
		return types.HijackedResponse{
			Conn:   &myConn{},
			Reader: bufio.NewReader(strings.NewReader("success")),
		}, nil
	}).Build().UnPatch()
	defer mockey.Mock((*client.Client).ContainerExecInspect).To(func(ctx context.Context, execID string) (container.ExecInspect, error) {
		assert.Equal(t, "test_exec_id", execID)
		return container.ExecInspect{
			ExitCode: 0,
		}, nil
	}).Build().UnPatch()

	content := "hello world"
	hdr := &tar.Header{
		Name: "name",
		Mode: 0644,
		Size: int64(len(content)),
	}
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)
	err := tw.WriteHeader(hdr)
	assert.NoError(t, err)
	_, err = tw.Write([]byte(content))
	assert.NoError(t, err)

	defer mockey.Mock((*client.Client).CopyFromContainer).To(func(ctx context.Context, containerID string, srcPath string) (io.ReadCloser, container.PathStat, error) {
		return ioutils.NewReadCloserWrapper(tarBuf, func() error {
			return nil
		}), container.PathStat{}, nil
	}).Build().UnPatch()

	sandbox := &DockerSandbox{
		config: Config{
			Timeout: 30 * time.Second,
		},
		client:      &client.Client{},
		containerID: "test_container_id",
	}

	result, err := sandbox.ReadFile(ctx, "./test")
	assert.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestDockerSandbox_SafeResolvePath(t *testing.T) {
	sandbox := &DockerSandbox{
		config: Config{
			WorkDir: "/workspace",
		},
	}

	path, err := sandbox.safeResolvePath("test.txt")
	assert.NoError(t, err)
	assert.Equal(t, "/workspace/test.txt", path)

	path, err = sandbox.safeResolvePath("/tmp/test.txt")
	assert.NoError(t, err)
	assert.Equal(t, "/tmp/test.txt", path)

	_, err = sandbox.safeResolvePath("../test.txt")
	assert.ErrorContains(t, err, "path contains potentially unsafe pattern")
}

type myConn struct {
}

func (m *myConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (m *myConn) Write(b []byte) (n int, err error) {
	return 0, nil
}

func (m *myConn) Close() error {
	return nil
}

func (m *myConn) LocalAddr() net.Addr {
	return nil
}

func (m *myConn) RemoteAddr() net.Addr {
	return nil
}

func (m *myConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *myConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *myConn) SetWriteDeadline(t time.Time) error {
	return nil
}
