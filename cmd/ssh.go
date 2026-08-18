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
	Use:   "ssh [name|vmid] [-- command...]",
	Short: "Connect to a pxon-managed container over SSH",
	Example: `  pxon ssh web-01
  pxon ssh web-01 -- uname -a
  pxon ssh -- hostname`,
	Args: validateSSHArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		invocation, err := parseSSHInvocation(args, cmd.ArgsLenAtDash())
		if err != nil {
			return err
		}

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
		if invocation.identifier != "" {
			match, err := findContainer(containers, invocation.identifier)
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

		return runSSH(ip, invocation.remoteCommand)
	},
}

type sshInvocation struct {
	identifier    string
	remoteCommand []string
}

func validateSSHArgs(cmd *cobra.Command, args []string) error {
	_, err := parseSSHInvocation(args, cmd.ArgsLenAtDash())
	return err
}

func parseSSHInvocation(args []string, argsLenAtDash int) (sshInvocation, error) {
	targetArgs := args
	invocation := sshInvocation{}

	if argsLenAtDash >= 0 {
		if argsLenAtDash > len(args) {
			return sshInvocation{}, fmt.Errorf("invalid argument separator position")
		}

		targetArgs = args[:argsLenAtDash]
		invocation.remoteCommand = append([]string(nil), args[argsLenAtDash:]...)
		if len(invocation.remoteCommand) == 0 || strings.TrimSpace(invocation.remoteCommand[0]) == "" {
			return sshInvocation{}, fmt.Errorf("remote command is required after --")
		}
	}

	if len(targetArgs) > 1 {
		if argsLenAtDash < 0 {
			return sshInvocation{}, fmt.Errorf("remote command arguments must follow --")
		}
		return sshInvocation{}, fmt.Errorf("accepts at most one container name or VMID before --")
	}

	if len(targetArgs) == 1 {
		invocation.identifier = strings.TrimSpace(targetArgs[0])
		if invocation.identifier == "" {
			return sshInvocation{}, fmt.Errorf("container name or VMID is required")
		}
	}

	return invocation, nil
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

func runSSH(ip string, remoteCommand []string) error {
	path, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh command not found: %w", err)
	}

	args := sshCommandArgs(ip, remoteCommand)
	return syscall.Exec(path, args, os.Environ())
}

func sshCommandArgs(ip string, remoteCommand []string) []string {
	args := make([]string, 0, 2+len(remoteCommand))
	args = append(args, "ssh", fmt.Sprintf("root@%s", ip))
	return append(args, remoteCommand...)
}
