package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"pxon/internal/config"
	"pxon/internal/network"
	"pxon/internal/proxmox"
	"pxon/internal/ui"
)

type containerCreateResult struct {
	*proxmox.TaskStatus
	Hostname  string              `json:"hostname"`
	VMID      int                 `json:"vmid"`
	Node      string              `json:"node"`
	Image     proxmox.Image       `json:"image"`
	RootFS    string              `json:"rootfs,omitempty"`
	Net0      string              `json:"net0,omitempty"`
	Stage     string              `json:"stage"`
	CloneTask *proxmox.TaskStatus `json:"clone_task,omitempty"`
	StartTask *proxmox.TaskStatus `json:"start_task,omitempty"`
	Error     string              `json:"error,omitempty"`
}

func createFromContainerImage(cmd *cobra.Command, client *proxmox.Client, cfg *config.Config, image proxmox.Image, vmid int, hostname string) error {
	for _, flag := range []string{"rootfs", "disk-size", "password", "ssh-key", "unprivileged"} {
		if cmd.Flags().Changed(flag) {
			return fmt.Errorf("--%s is not supported for container images: clones inherit disk size, credentials, and privilege mode from the image", flag)
		}
	}
	if createOpts.memory <= 0 || createOpts.cores <= 0 || createOpts.swap < 0 {
		return fmt.Errorf("memory and cores must be positive and swap cannot be negative")
	}
	request := proxmox.CloneContainerRequest{Node: image.Node, SourceVMID: image.VMID, VMID: vmid, Hostname: hostname, Storage: strings.TrimSpace(createOpts.storage)}
	if request.Storage == "" {
		request.Storage = strings.TrimSpace(cfg.DefaultStorage)
	}
	if err := request.Validate(); err != nil {
		return err
	}
	net0, err := resolveCreateNetwork(client, cfg, image.Node)
	if err != nil {
		return err
	}
	result := containerCreateResult{Hostname: hostname, VMID: vmid, Node: image.Node, Image: image, Net0: net0, Stage: "clone"}
	finish := func(operationErr error) error {
		if operationErr != nil {
			result.Error = fmt.Sprintf("container %s (VMID %d, node %s), stage %s: %v; inspect the destination before retrying", hostname, vmid, image.Node, result.Stage, operationErr)
			operationErr = fmt.Errorf("%s", result.Error)
		}
		if jsonEnabled() {
			return writeJSONResult(cmd, result, operationErr)
		}
		if operationErr == nil {
			ui.RenderCreateSummary(cmd.OutOrStdout(), ui.CreateSummary{Hostname: hostname, VMID: vmid, Node: image.Node, Status: result.ExitStatus, Template: image.ID, RootFS: result.RootFS, Net0: net0, UPID: result.UPID})
		}
		return operationErr
	}
	upid, err := client.StartCloneContainer(request)
	if err != nil {
		return finish(err)
	}
	result.TaskStatus = &proxmox.TaskStatus{UPID: upid, Node: image.Node}
	if !jsonEnabled() {
		fmt.Fprintf(cmd.ErrOrStderr(), "Cloning %s as %s (VMID %d), task %s...\n", image.ID, hostname, vmid, upid)
	}
	result.CloneTask, err = waitForImageTask(client, image.Node, upid)
	if result.CloneTask != nil {
		result.TaskStatus = result.CloneTask
	}
	if err != nil {
		return finish(err)
	}
	result.Stage = "configure"
	cloneConfig, err := client.ContainerConfig(image.Node, vmid)
	if err != nil {
		return finish(err)
	}
	result.RootFS = cloneConfig.RootFS
	values := isolatedNetworkConfig(cloneConfig, true)
	values.Set("hostname", hostname)
	values.Set("net0", net0)
	values.Set("tags", proxmox.JoinManagedTags(createOpts.tags))
	for flag, value := range map[string]int{"memory": createOpts.memory, "cores": createOpts.cores, "swap": createOpts.swap} {
		if cmd.Flags().Changed(flag) {
			values.Set(flag, strconv.Itoa(value))
		}
	}
	if err := client.UpdateContainer(image.Node, vmid, values); err != nil {
		return finish(err)
	}
	if createOpts.start {
		result.Stage = "start"
		upid, err = client.StartContainer(image.Node, vmid)
		if err != nil {
			return finish(err)
		}
		result.TaskStatus = &proxmox.TaskStatus{UPID: upid, Node: image.Node}
		result.StartTask, err = waitForImageTask(client, image.Node, upid)
		if result.StartTask != nil {
			result.TaskStatus = result.StartTask
		}
		if err != nil {
			return finish(err)
		}
	}
	result.Stage = "completed"
	return finish(nil)
}

func resolveCreateNetwork(client *proxmox.Client, cfg *config.Config, node string) (string, error) {
	if value := strings.TrimSpace(createOpts.net0); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(cfg.DefaultNet0); value != "" {
		return value, nil
	}
	used, err := client.UsedIPv4Addresses(node)
	if err != nil {
		return "", err
	}
	net0, err := network.BuildNet0(cfg.Network, used)
	if err != nil {
		return "", fmt.Errorf("missing network configuration: provide --net0 or configure networking with pxon config: %w", err)
	}
	return net0, nil
}
