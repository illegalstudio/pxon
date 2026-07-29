package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"pxon/internal/knownhosts"
	"pxon/internal/proxmox"
	"pxon/internal/ui"
	"pxon/internal/wizard"
)

type deleteOptions struct {
	force bool
}

type deleteResult struct {
	Name              string              `json:"name"`
	VMID              int                 `json:"vmid"`
	Node              string              `json:"node"`
	IP                string              `json:"ip,omitempty"`
	Task              *proxmox.TaskStatus `json:"task"`
	KnownHostsFound   int                 `json:"known_hosts_found"`
	KnownHostsRemoved bool                `json:"known_hosts_removed"`
}

var deleteOpts deleteOptions

var deleteCmd = &cobra.Command{
	Use:   "delete [name|vmid]",
	Short: "Delete a pxon-managed LXC container",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := currentConfig()
		if err != nil {
			return err
		}

		client, err := proxmox.NewClient(cfg)
		if err != nil {
			return err
		}

		containers, err := client.ManagedContainers()
		if err != nil {
			return err
		}

		if len(containers) == 0 {
			return fmt.Errorf("no pxon-managed containers available")
		}

		var target proxmox.Container
		if len(args) == 1 {
			target, err = findContainer(containers, args[0])
		} else {
			target, err = pickContainer(containers)
		}
		if err != nil {
			return err
		}

		knownHostsPath, err := knownhosts.DefaultPath()
		if err != nil {
			return err
		}

		knownHostMatches, err := knownhosts.Find(knownHostsPath, []string{target.Name, target.IP})
		if err != nil {
			return err
		}

		if !deleteOpts.force {
			confirmed, err := wizard.Confirm(
				fmt.Sprintf(
					"Permanently delete %s (VMID %d, node %s, status %s)?",
					target.Name,
					target.VMID,
					target.Node,
					target.Status,
				),
				false,
			)
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Fprintln(cmd.OutOrStdout(), "Operation cancelled.")
				return nil
			}
		}

		removeKnownHosts := deleteOpts.force
		if knownHostMatches.Rows > 0 && !deleteOpts.force {
			removeKnownHosts, err = wizard.Confirm(
				fmt.Sprintf(
					"Found %s in %s for %s. Remove after deleting the container?",
					knownHostsEntryCount(knownHostMatches.Rows),
					knownHostsPath,
					strings.Join(knownHostMatches.Hosts, ", "),
				),
				false,
			)
			if err != nil {
				return err
			}
		}

		progressWriter := cmd.ErrOrStderr()
		if jsonEnabled() {
			progressWriter = io.Discard
		}
		fmt.Fprintf(progressWriter, "Deleting %s (VMID %d)...\n", target.Name, target.VMID)

		apiForce := deleteOpts.force || strings.EqualFold(target.Status, "running")
		upid, err := client.StartDeleteContainer(target.Node, target.VMID, apiForce)
		if err != nil {
			return err
		}

		taskStatus, err := client.WaitForTask(target.Node, upid, 2*time.Minute)
		if err != nil {
			return err
		}
		if taskStatus.ExitStatus != "OK" {
			return fmt.Errorf("Proxmox task failed: %s", taskStatus.ExitStatus)
		}

		knownHostsRemoved := false
		if knownHostMatches.Rows > 0 && removeKnownHosts {
			if err := knownhosts.Remove(knownHostsPath, knownHostMatches.Hosts); err != nil {
				return fmt.Errorf("container deleted, but known_hosts cleanup failed: %w", err)
			}
			knownHostsRemoved = true
		}

		result := deleteResult{
			Name:              target.Name,
			VMID:              target.VMID,
			Node:              target.Node,
			IP:                target.IP,
			Task:              taskStatus,
			KnownHostsFound:   knownHostMatches.Rows,
			KnownHostsRemoved: knownHostsRemoved,
		}

		if jsonEnabled() {
			return ui.WriteJSON(cmd.OutOrStdout(), result)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Container %s (VMID %d) deleted.\n", target.Name, target.VMID)
		switch {
		case knownHostsRemoved:
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s from %s.\n", knownHostsEntryCount(knownHostMatches.Rows), knownHostsPath)
		case knownHostMatches.Rows > 0:
			fmt.Fprintf(cmd.OutOrStdout(), "Kept %s in %s.\n", knownHostsEntryCount(knownHostMatches.Rows), knownHostsPath)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVar(
		&deleteOpts.force,
		"force",
		false,
		"Delete without confirmation and automatically remove matching SSH known_hosts entries",
	)
}

func knownHostsEntryCount(count int) string {
	if count == 1 {
		return "1 entry"
	}

	return fmt.Sprintf("%d entries", count)
}
