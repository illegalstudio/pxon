package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"pxon/internal/proxmox"
)

type lifecycleResult struct {
	Operation string              `json:"operation"`
	Name      string              `json:"name"`
	VMID      int                 `json:"vmid"`
	Node      string              `json:"node"`
	IP        string              `json:"ip,omitempty"`
	Status    string              `json:"status,omitempty"`
	Uptime    int64               `json:"uptime"`
	Changed   bool                `json:"changed"`
	UPID      string              `json:"upid,omitempty"`
	Task      *proxmox.TaskStatus `json:"task,omitempty"`
	Error     string              `json:"error,omitempty"`
}

func newLifecycleCommand(operation string) *cobra.Command {
	var timeout, shutdownTimeout time.Duration
	command := &cobra.Command{
		Use:   operation + " <name|vmid>",
		Short: map[string]string{"start": "Start a managed container if it is stopped", "stop": "Cleanly shut down a managed container if it is running", "status": "Read the current state of a managed container"}[operation],
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeout <= 0 {
				return fmt.Errorf("--timeout must be positive")
			}
			if operation == "stop" && shutdownTimeout <= 0 {
				return fmt.Errorf("--shutdown-timeout must be positive")
			}
			cfg, err := currentConfig()
			if err != nil {
				return err
			}
			client, err := proxmox.NewClient(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			client = client.WithContext(ctx)
			containers, err := client.ManagedContainers()
			if err != nil {
				return err
			}
			target, err := findContainer(containers, args[0])
			if err != nil {
				return err
			}
			if target.Template == 1 {
				return fmt.Errorf("%s (VMID %d) is a template; create a container with --image lxc:%d first", target.Name, target.VMID, target.VMID)
			}
			result := lifecycleResult{Operation: operation, Name: target.Name, VMID: target.VMID, Node: target.Node, IP: target.IP}
			operationErr := runLifecycle(client, &result, timeout, shutdownTimeout)
			if operationErr != nil {
				result.Error = operationErr.Error()
			}
			if jsonEnabled() {
				return writeJSONResult(cmd, result, operationErr)
			}
			if operationErr != nil {
				if result.UPID != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "Proxmox task: %s (node %s). Check its outcome before retrying.\n", result.UPID, result.Node)
				}
				return operationErr
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (VMID %d, node %s): %s\n", result.Name, result.VMID, result.Node, result.Status)
			return nil
		},
	}
	command.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Overall deadline including lookup, requests, and task polling")
	if operation == "stop" {
		command.Flags().DurationVar(&shutdownTimeout, "shutdown-timeout", time.Minute, "Time Proxmox allows for clean shutdown; never forces a stop")
	}
	return command
}

func runLifecycle(client *proxmox.Client, result *lifecycleResult, timeout, shutdownTimeout time.Duration) error {
	state, err := client.ContainerState(result.Node, result.VMID)
	if err != nil {
		return err
	}
	result.Status, result.Uptime = state.Status, state.Uptime
	if result.Operation == "status" {
		return nil
	}
	if state.Status != "running" && state.Status != "stopped" {
		return fmt.Errorf("cannot %s a container with status %q", result.Operation, state.Status)
	}
	wanted := "running"
	if result.Operation == "stop" {
		wanted = "stopped"
	}
	if state.Status == wanted {
		return nil
	}
	if result.Operation == "start" {
		result.UPID, err = client.StartContainer(result.Node, result.VMID)
	} else {
		result.UPID, err = client.ShutdownContainer(result.Node, result.VMID, shutdownTimeout)
	}
	if err != nil {
		return err
	}
	result.Changed = true
	result.Task, err = client.WaitForTask(result.Node, result.UPID, timeout)
	if err != nil {
		return err
	}
	if result.Task.ExitStatus != "OK" {
		return fmt.Errorf("Proxmox task failed: %s", result.Task.ExitStatus)
	}
	state, err = client.ContainerState(result.Node, result.VMID)
	if err != nil {
		return err
	}
	result.Status, result.Uptime = state.Status, state.Uptime
	if state.Status != wanted {
		return fmt.Errorf("Proxmox task finished but container status is %q, expected %q", state.Status, wanted)
	}
	return nil
}

func init() {
	for _, operation := range []string{"start", "stop", "status"} {
		rootCmd.AddCommand(newLifecycleCommand(operation))
	}
}
