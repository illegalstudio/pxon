package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"pxon/internal/proxmox"
	"pxon/internal/wizard"
)

var sshCmd = &cobra.Command{
	Use:   "ssh [name|vmid]",
	Short: "Open an SSH session to a pxon-managed container",
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
			name := strings.TrimSpace(args[0])
			match, err := findContainer(containers, name)
			if err != nil {
				return err
			}
			target = match
		} else {
			match, err := pickContainer(containers)
			if err != nil {
				return err
			}
			target = match
		}

		ip := strings.TrimSpace(target.IP)
		if ip == "" {
			return fmt.Errorf("container %s has no configured IP address", target.Name)
		}

		return runSSH(ip)
	},
}

func init() {
	rootCmd.AddCommand(sshCmd)
}

func findContainer(containers []proxmox.Container, identifier string) (proxmox.Container, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return proxmox.Container{}, fmt.Errorf("container name or VMID is required")
	}

	if vmid, err := strconv.Atoi(identifier); err == nil {
		for _, container := range containers {
			if container.VMID == vmid {
				return container, nil
			}
		}

		return proxmox.Container{}, fmt.Errorf("container with VMID %d not found", vmid)
	}

	var matches []proxmox.Container
	for _, c := range containers {
		if strings.EqualFold(c.Name, identifier) {
			matches = append(matches, c)
		}
	}

	switch len(matches) {
	case 0:
		return proxmox.Container{}, fmt.Errorf("container %q not found", identifier)
	case 1:
		return matches[0], nil
	default:
		return proxmox.Container{}, fmt.Errorf("multiple containers named %q found; specify the VMID", identifier)
	}
}

func pickContainer(containers []proxmox.Container) (proxmox.Container, error) {
	options := make([]string, len(containers))
	for i, c := range containers {
		ip := c.IP
		if ip == "" {
			ip = "no IP"
		}
		options[i] = fmt.Sprintf("%s  (%d, %s, %s)", c.Name, c.VMID, ip, c.Status)
	}

	choice, err := wizard.Select("Select a container", options, 0)
	if err != nil {
		return proxmox.Container{}, err
	}

	for i, o := range options {
		if o == choice {
			return containers[i], nil
		}
	}

	return proxmox.Container{}, fmt.Errorf("invalid selection")
}

func runSSH(ip string) error {
	path, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh command not found: %w", err)
	}

	args := []string{"ssh", fmt.Sprintf("root@%s", ip)}
	return syscall.Exec(path, args, os.Environ())
}
