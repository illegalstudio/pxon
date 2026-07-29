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
	Use:   "ssh [nome]",
	Short: "Apre una sessione SSH verso un container gestito da pxon",
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
			return fmt.Errorf("nessun container pxon disponibile")
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
			return fmt.Errorf("container %s non ha un indirizzo IP configurato", target.Name)
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
		return proxmox.Container{}, fmt.Errorf("nome o VMID del container richiesto")
	}

	if vmid, err := strconv.Atoi(identifier); err == nil {
		for _, container := range containers {
			if container.VMID == vmid {
				return container, nil
			}
		}

		return proxmox.Container{}, fmt.Errorf("container con VMID %d non trovato", vmid)
	}

	var matches []proxmox.Container
	for _, c := range containers {
		if strings.EqualFold(c.Name, identifier) {
			matches = append(matches, c)
		}
	}

	switch len(matches) {
	case 0:
		return proxmox.Container{}, fmt.Errorf("container %q non trovato", identifier)
	case 1:
		return matches[0], nil
	default:
		return proxmox.Container{}, fmt.Errorf("trovati più container con nome %q, specifica il VMID", identifier)
	}
}

func pickContainer(containers []proxmox.Container) (proxmox.Container, error) {
	options := make([]string, len(containers))
	for i, c := range containers {
		ip := c.IP
		if ip == "" {
			ip = "senza IP"
		}
		options[i] = fmt.Sprintf("%s  (%d, %s, %s)", c.Name, c.VMID, ip, c.Status)
	}

	choice, err := wizard.Select("Seleziona il container", options, 0)
	if err != nil {
		return proxmox.Container{}, err
	}

	for i, o := range options {
		if o == choice {
			return containers[i], nil
		}
	}

	return proxmox.Container{}, fmt.Errorf("selezione non valida")
}

func runSSH(ip string) error {
	path, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("comando ssh non trovato: %w", err)
	}

	args := []string{"ssh", fmt.Sprintf("root@%s", ip)}
	return syscall.Exec(path, args, os.Environ())
}
