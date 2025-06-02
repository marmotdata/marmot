package utils

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	imageTypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
)

type ContainerManager struct {
	client     *client.Client
	ctx        context.Context
	networkID  string
	containers []string
}

func NewContainerManager(ctx context.Context) (*ContainerManager, error) {
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.43"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating Docker client: %w", err)
	}

	cm := &ContainerManager{
		client: dockerClient,
		ctx:    ctx,
	}

	// Use context for network creation
	networkID, err := createNetwork(cm.ctx, dockerClient, NewDefaultConfig().NetworkName)
	if err != nil {
		dockerClient.Close()
		return nil, err
	}
	cm.networkID = networkID
	return cm, nil
}

func (cm *ContainerManager) ExecCommand(containerID string, cmd []string) (string, error) {
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Create the exec instance
	resp, err := cm.client.ContainerExecCreate(cm.ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Start the exec instance
	resp2, err := cm.client.ContainerExecAttach(cm.ctx, resp.ID, container.ExecStartOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp2.Close()

	// Read the output
	output, err := io.ReadAll(resp2.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to read exec output: %w", err)
	}

	// Wait for the exec to finish and get the exit code
	var exitCode int
	for {
		inspect, err := cm.client.ContainerExecInspect(cm.ctx, resp.ID)
		if err != nil {
			return "", fmt.Errorf("failed to inspect exec: %w", err)
		}
		if !inspect.Running {
			exitCode = inspect.ExitCode
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if exitCode != 0 {
		return string(output), fmt.Errorf("command exited with code %d", exitCode)
	}

	return string(output), nil
}

func (cm *ContainerManager) BuildMarmot(ctx context.Context, projectRoot string) error {
	log.Println("Building Marmot image...")
	buildContext := projectRoot
	dockerfile := "Dockerfile"

	tar, err := archive.TarWithOptions(buildContext, &archive.TarOptions{
		ExcludePatterns: []string{"node_modules", "build"},
	})
	if err != nil {
		return fmt.Errorf("creating tar: %w", err)
	}

	opts := types.ImageBuildOptions{
		Dockerfile: dockerfile,
		Tags:       []string{"marmot:test"},
		Remove:     true,
	}

	resp, err := cm.client.ImageBuild(ctx, tar, opts)
	if err != nil {
		return fmt.Errorf("building image: %w", err)
	}
	defer resp.Body.Close()

	buildOutput, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading build output: %w", err)
	}
	log.Printf("Build output: %s", string(buildOutput))
	log.Println("Marmot image built successfully")
	return nil
}

func createNetwork(ctx context.Context, dockerClient *client.Client, networkName string) (string, error) {
	// First check if network exists
	networks, err := dockerClient.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing networks: %w", err)
	}

	for _, network := range networks {
		if network.Name == networkName {
			return network.ID, nil
		}
	}

	// Create if doesn't exist
	network, err := dockerClient.NetworkCreate(ctx, networkName, types.NetworkCreate{
		Driver: "bridge",
	})
	if err != nil {
		return "", fmt.Errorf("creating network: %w", err)
	}
	return network.ID, nil
}

func (cm *ContainerManager) Close() {
	log.Println("Cleaning up resources...")
	for _, containerID := range cm.containers {
		if err := cm.CleanupContainer(containerID); err != nil {
			log.Printf("Failed to cleanup container %s: %v\n", containerID, err)
		}
	}

	if cm.networkID != "" {
		if err := cm.client.NetworkRemove(cm.ctx, cm.networkID); err != nil {
			log.Printf("Failed to remove network: %v\n", err)
		}
	}
	log.Println("Cleanup completed")
}

func (cm *ContainerManager) StartPostgres(config TestConfig) (string, error) {
	log.Println("Starting Postgres container...")
	postgresConfig := &container.Config{
		Image: "postgres:17",
		Env: []string{
			"POSTGRES_PASSWORD=" + config.PostgresPassword,
			"POSTGRES_DB=test",
		},
		ExposedPorts: nat.PortSet{
			nat.Port("5432/tcp"): struct{}{},
		},
	}

	postgresHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(config.NetworkName),
		PortBindings: nat.PortMap{
			nat.Port("5432/tcp"): []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: config.PostgresPort},
			},
		},
	}

	return cm.StartContainer(postgresConfig, postgresHostConfig, "postgres-test")
}

func (cm *ContainerManager) StartMarmotContainer(config TestConfig) (string, error) {
	log.Println("Starting Marmot container...")
	appConfig := &container.Config{
		Image: "marmot:test",
		Env: []string{
			"MARMOT_DATABASE_HOST=postgres-test",
			"MARMOT_DATABASE_PORT=5432",
			"MARMOT_DATABASE_USER=postgres",
			fmt.Sprintf("MARMOT_DATABASE_PASSWORD=%s", config.PostgresPassword),
			"MARMOT_DATABASE_NAME=test",
			"MARMOT_DATABASE_SSLMODE=disable",
			"MARMOT_DATABASE_MAX_CONNS=10",
			"MARMOT_DATABASE_IDLE_CONNS=5",
			"MARMOT_DATABASE_CONN_LIFETIME=300",
		},
		ExposedPorts: nat.PortSet{
			nat.Port("8080/tcp"): struct{}{},
		},
	}

	appHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(config.NetworkName),
		PortBindings: nat.PortMap{
			nat.Port("8080/tcp"): []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: config.ApplicationPort},
			},
		},
	}

	return cm.StartContainer(appConfig, appHostConfig, "marmot-test")
}

func (cm *ContainerManager) imageExists(imageName string) (bool, error) {
	images, err := cm.client.ImageList(cm.ctx, imageTypes.ListOptions{})
	if err != nil {
		return false, err
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == imageName {
				return true, nil
			}
		}
	}
	return false, nil
}

func (cm *ContainerManager) PullImage(image string) error {
	exists, err := cm.imageExists(image)
	if err != nil {
		return fmt.Errorf("checking image existence: %w", err)
	}

	if exists {
		log.Printf("Image %s already exists locally, skipping pull", image)
		return nil
	}

	log.Printf("Pulling image: %s", image)
	reader, err := cm.client.ImagePull(cm.ctx, image, imageTypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", image, err)
	}
	defer reader.Close()

	// Read the output to complete the pull
	pullOutput, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("reading pull output: %w", err)
	}
	log.Printf("Pull output: %s", string(pullOutput))
	return nil
}

func (cm *ContainerManager) StartContainer(config *container.Config, hostConfig *container.HostConfig, name string) (string, error) {
	log.Printf("Starting container: %s", name)

	// Skip pulling for local images (like marmot:test)
	if !strings.Contains(config.Image, ":test") {
		if err := cm.PullImage(config.Image); err != nil {
			return "", err
		}
	}

	resp, err := cm.client.ContainerCreate(cm.ctx, config, hostConfig, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}

	if err := cm.client.ContainerStart(cm.ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("starting container: %w", err)
	}

	if showLogs := os.Getenv("CONTAINER_LOGS"); showLogs == "true" {
		logs, err := cm.client.ContainerLogs(cm.ctx, resp.ID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     false,
		})
		if err != nil {
			log.Printf("Warning: Failed to get container logs: %v", err)
		} else {
			defer logs.Close()
			logContent, _ := io.ReadAll(logs)
			log.Printf("Container %s logs: %s", name, string(logContent))
		}
	}

	cm.containers = append(cm.containers, resp.ID)
	log.Printf("Container started successfully: %s (%s)", name, resp.ID)
	return resp.ID, nil
}

func (cm *ContainerManager) CleanupContainer(containerID string) error {
	log.Printf("Cleaning up container: %s", containerID)

	if showLogs := os.Getenv("CONTAINER_LOGS"); showLogs == "true" {
		logs, err := cm.client.ContainerLogs(cm.ctx, containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     false,
		})
		if err == nil {
			defer logs.Close()
			logContent, _ := io.ReadAll(logs)
			log.Printf("Container %s final logs: %s", containerID, string(logContent))
		}
	}

	if err := cm.client.ContainerStop(cm.ctx, containerID, container.StopOptions{}); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}
	if err := cm.client.ContainerRemove(cm.ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("removing container: %w", err)
	}
	log.Printf("Container %s cleaned up successfully", containerID)
	return nil
}

func (cm *ContainerManager) RunMarmotCommandWithConfig(config TestConfig, command []string, configContent string, volumeMounts ...string) error {
	tmpDir, err := os.MkdirTemp("", "marmot-config-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return err
	}

	containerConfig := &container.Config{
		Image:      "marmot:test",
		Cmd:        command,
		WorkingDir: "/tmp",
	}

	// Start with the config file bind
	binds := []string{fmt.Sprintf("%s:/tmp/config.yaml:ro", configFile)}

	// Add any additional volume mounts
	binds = append(binds, volumeMounts...)

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(config.NetworkName),
		Binds:       binds,
	}

	containerID, err := cm.StartContainer(containerConfig, hostConfig, "")
	if err != nil {
		return err
	}
	defer cm.CleanupContainer(containerID)

	statusCh, errCh := cm.client.ContainerWait(cm.ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return err
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("command failed with status code %d", status.StatusCode)
		}
	}
	return nil
}
