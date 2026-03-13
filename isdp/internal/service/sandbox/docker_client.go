package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerClient Docker客户端封装
type DockerClient struct {
	client *client.Client
}

// NewDockerClient 创建Docker客户端
func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerClient{client: cli}, nil
}

// ContainerConfig 容器配置
type ContainerConfig struct {
	Image       string
	Cmd         []string
	Env         []string
	WorkDir     string
	MemoryLimit int64  // MB
	CPUQuota    int64  // CPU quota
	Timeout     time.Duration
	Mounts      []MountConfig
}

// MountConfig 挂载配置
type MountConfig struct {
	Source   string
	Target   string
	ReadOnly bool
}

// CreateContainer 创建容器
func (d *DockerClient) CreateContainer(ctx context.Context, name string, config *ContainerConfig) (string, error) {
	// 拉取镜像
	reader, err := d.client.ImagePull(ctx, config.Image, types.ImagePullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	// 构建挂载
	var mounts []mount.Mount
	for _, m := range config.Mounts {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.Source,
			Target:   m.Target,
			ReadOnly: m.ReadOnly,
		})
	}

	// 创建容器
	containerConfig := &container.Config{
		Image:      config.Image,
		Cmd:        config.Cmd,
		Env:        config.Env,
		WorkingDir: config.WorkDir,
		Tty:        false,
	}

	hostConfig := &container.HostConfig{
		Mounts: mounts,
		Resources: container.Resources{
			Memory:    config.MemoryLimit * 1024 * 1024, // MB to bytes
			CPUQuota:  config.CPUQuota,
		},
		AutoRemove: false,
	}

	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

// StartContainer 启动容器
func (d *DockerClient) StartContainer(ctx context.Context, containerID string) error {
	return d.client.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer 停止容器
func (d *DockerClient) StopContainer(ctx context.Context, containerID string, timeout *int) error {
	return d.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: timeout})
}

// RemoveContainer 移除容器
func (d *DockerClient) RemoveContainer(ctx context.Context, containerID string) error {
	return d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{})
}

// ExecInContainer 在容器中执行命令
func (d *DockerClient) ExecInContainer(ctx context.Context, containerID string, cmd []string) (string, string, error) {
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}

	execResp, err := d.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", "", err
	}

	attachResp, err := d.client.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", "", err
	}
	defer attachResp.Close()

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
	if err != nil {
		return "", "", err
	}

	return stdout.String(), stderr.String(), nil
}

// GetContainerLogs 获取容器日志
func (d *DockerClient) GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}

	reader, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var buf bytes.Buffer
	_, err = stdcopy.StdCopy(&buf, &buf, reader)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// CopyToContainer 复制文件到容器
func (d *DockerClient) CopyToContainer(ctx context.Context, containerID string, path string, content []byte) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdr := &tar.Header{
		Name: filepath.Base(path),
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}

	return d.client.CopyToContainer(ctx, containerID, filepath.Dir(path), &buf, types.CopyToContainerOptions{})
}

// CopyFromContainer 从容器复制文件
func (d *DockerClient) CopyFromContainer(ctx context.Context, containerID string, path string) ([]byte, error) {
	reader, _, err := d.client.CopyFromContainer(ctx, containerID, path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hdr.Typeflag == tar.TypeReg {
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, tr); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("file not found: %s", path)
}

// WaitContainer 等待容器结束
func (d *DockerClient) WaitContainer(ctx context.Context, containerID string) (int64, error) {
	statusCh, errCh := d.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return -1, err
	case status := <-statusCh:
		return status.StatusCode, nil
	}
}

// ListContainers 列出容器
func (d *DockerClient) ListContainers(ctx context.Context, all bool) ([]types.Container, error) {
	return d.client.ContainerList(ctx, container.ListOptions{All: all})
}

// Close 关闭客户端
func (d *DockerClient) Close() error {
	return d.client.Close()
}