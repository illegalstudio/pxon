package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"pxon/internal/network"
	"pxon/internal/proxmox"
)

type createOptions struct {
	node         string
	vmid         int
	template     string
	rootfs       string
	storage      string
	diskSize     string
	password     string
	memory       int
	cores        int
	swap         int
	net0         string
	start        bool
	unprivileged bool
	tags         []string
}

var createOpts createOptions

var createCmd = &cobra.Command{
	Use:   "create <hostname>",
	Short: "Crea un container LXC gestito da pxon",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := currentConfig()
		if err != nil {
			return err
		}

		client, err := proxmox.NewClient(cfg)
		if err != nil {
			return err
		}

		node := strings.TrimSpace(createOpts.node)
		if node == "" {
			node, err = client.DefaultNode()
			if err != nil {
				return err
			}
		}

		vmid := createOpts.vmid
		if vmid == 0 {
			vmid, err = client.NextID()
			if err != nil {
				return err
			}
		}

		rootfs := strings.TrimSpace(createOpts.rootfs)
		if rootfs == "" {
			storage := strings.TrimSpace(createOpts.storage)
			if storage == "" {
				storage = strings.TrimSpace(cfg.DefaultStorage)
			}

			diskSize := strings.TrimSpace(createOpts.diskSize)
			if diskSize == "" {
				diskSize = strings.TrimSpace(cfg.DefaultDiskSize)
			}

			rootfs, err = proxmox.BuildRootFS(storage, diskSize)
			if err != nil {
				return err
			}
		}

		template := strings.TrimSpace(createOpts.template)
		if template == "" {
			template = strings.TrimSpace(cfg.DefaultImage)
		}

		password := createOpts.password
		if password == "" {
			password = cfg.DefaultPassword
		}

		net0 := strings.TrimSpace(createOpts.net0)
		if net0 == "" {
			if strings.TrimSpace(cfg.DefaultNet0) != "" {
				net0 = strings.TrimSpace(cfg.DefaultNet0)
			} else {
				usedIPs, err := client.UsedIPv4Addresses(node)
				if err != nil {
					return err
				}

				net0, err = network.BuildNet0(cfg.Network, usedIPs)
				if err != nil {
					return fmt.Errorf("missing network configuration: provide --net0 or configure networking with `pxon config`: %w", err)
				}
			}
		}

		req := proxmox.CreateContainerRequest{
			Node:         node,
			VMID:         vmid,
			Hostname:     strings.TrimSpace(args[0]),
			OSTemplate:   template,
			RootFS:       rootfs,
			Password:     password,
			Memory:       createOpts.memory,
			Cores:        createOpts.cores,
			Swap:         createOpts.swap,
			Net0:         net0,
			Start:        createOpts.start,
			Unprivileged: createOpts.unprivileged,
			Tags:         createOpts.tags,
		}

		upid, err := client.StartCreateContainer(req)
		if err != nil {
			return err
		}

		taskStatus, err := waitForCreateTask(cmd.ErrOrStderr(), client, node, upid, req.Hostname, vmid, req.Start)
		if err != nil {
			return err
		}

		data, err := json.Marshal(taskStatus)
		if err != nil {
			return fmt.Errorf("encode Proxmox task result: %w", err)
		}

		var pretty bytes.Buffer
		if err := json.Indent(&pretty, data, "", "  "); err != nil {
			return fmt.Errorf("invalid JSON response from Proxmox: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), pretty.String())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVar(&createOpts.node, "node", "", "Nodo Proxmox su cui creare il container")
	createCmd.Flags().IntVar(&createOpts.vmid, "vmid", 0, "VMID da usare; se omesso viene richiesto il prossimo ID disponibile")
	createCmd.Flags().StringVar(&createOpts.template, "template", "", "Template LXC Proxmox; se omesso usa il default configurato")
	createCmd.Flags().StringVar(&createOpts.rootfs, "rootfs", "", "Valore rootfs Proxmox completo, ad esempio storage:8")
	createCmd.Flags().StringVar(&createOpts.storage, "storage", "", "Storage Proxmox per il disco rootfs; se omesso usa il default configurato")
	createCmd.Flags().StringVar(&createOpts.diskSize, "disk-size", "", "Dimensione disco rootfs; se omessa usa il default configurato")
	createCmd.Flags().StringVar(&createOpts.password, "password", "", "Password root iniziale; se omessa usa la default configurata")
	createCmd.Flags().IntVar(&createOpts.memory, "memory", 512, "Memoria RAM in MB")
	createCmd.Flags().IntVar(&createOpts.cores, "cores", 1, "Numero di CPU core")
	createCmd.Flags().IntVar(&createOpts.swap, "swap", 512, "Swap in MB")
	createCmd.Flags().StringVar(&createOpts.net0, "net0", "", "Configurazione rete Proxmox net0; se omessa usa il default configurato")
	createCmd.Flags().BoolVar(&createOpts.start, "start", true, "Avvia il container al termine della creazione; usa --start=false per non avviarlo")
	createCmd.Flags().BoolVar(&createOpts.unprivileged, "unprivileged", true, "Crea un container LXC unprivileged")
	createCmd.Flags().StringSliceVar(&createOpts.tags, "tag", nil, "Tag aggiuntivo da applicare; il tag pxon viene sempre aggiunto")
}

func waitForCreateTask(writer io.Writer, client *proxmox.Client, node, upid, hostname string, vmid int, shouldStart bool) (*proxmox.TaskStatus, error) {
	const (
		pollInterval    = 2 * time.Second
		spinnerInterval = 150 * time.Millisecond
		timeout         = 2 * time.Minute
	)

	spinnerFrames := []string{"|", "/", "-", "\\"}
	spinnerIndex := 0
	lastLogN := 0
	startedAt := time.Now()
	statusText := fmt.Sprintf("creating %s (vmid %d)", hostname, vmid)
	deadline := startedAt.Add(timeout)

	render := func() {
		fmt.Fprintf(writer, "\r%s %s [%ds]", spinnerFrames[spinnerIndex], statusText, int(time.Since(startedAt).Seconds()))
		spinnerIndex = (spinnerIndex + 1) % len(spinnerFrames)
	}

	printLogs := func(entries []proxmox.TaskLogEntry) {
		for _, entry := range entries {
			if entry.N <= lastLogN {
				continue
			}

			line := strings.TrimSpace(entry.T)
			if line == "" || line == "no content" {
				lastLogN = entry.N
				continue
			}

			fmt.Fprint(writer, "\r\033[K")
			fmt.Fprintf(writer, "task: %s\n", line)
			lastLogN = entry.N
		}
	}

	poll := func() (*proxmox.TaskStatus, error) {
		logs, err := client.TaskLog(node, upid)
		if err != nil {
			return nil, err
		}
		printLogs(logs)

		taskStatus, err := client.TaskStatus(node, upid)
		if err != nil {
			return nil, err
		}

		statusText = fmt.Sprintf("creating %s (vmid %d) on %s", hostname, vmid, node)
		if taskStatus.Status == "stopped" {
			if taskStatus.ExitStatus == "OK" {
				if shouldStart {
					statusText = fmt.Sprintf("container %s created and started as %d", hostname, vmid)
				} else {
					statusText = fmt.Sprintf("container %s created as %d", hostname, vmid)
				}
			} else {
				statusText = fmt.Sprintf("container %s failed", hostname)
			}
		}

		return taskStatus, nil
	}

	taskStatus, err := poll()
	if err != nil {
		fmt.Fprint(writer, "\r\033[K")
		return nil, err
	}

	if taskStatus.Status == "stopped" {
		fmt.Fprint(writer, "\r\033[K")
		if taskStatus.ExitStatus != "OK" {
			return nil, fmt.Errorf("Proxmox task failed: %s", taskStatus.ExitStatus)
		}
		fmt.Fprintf(writer, "done: %s\n", statusText)
		return taskStatus, nil
	}

	spinnerTicker := time.NewTicker(spinnerInterval)
	pollTicker := time.NewTicker(pollInterval)
	defer spinnerTicker.Stop()
	defer pollTicker.Stop()

	render()

	for {
		select {
		case <-spinnerTicker.C:
			render()
		case <-pollTicker.C:
			taskStatus, err = poll()
			if err != nil {
				fmt.Fprint(writer, "\r\033[K")
				return nil, err
			}

			if taskStatus.Status == "stopped" {
				fmt.Fprint(writer, "\r\033[K")
				if taskStatus.ExitStatus != "OK" {
					return nil, fmt.Errorf("Proxmox task failed: %s", taskStatus.ExitStatus)
				}

				fmt.Fprintf(writer, "done: %s\n", statusText)
				return taskStatus, nil
			}

			if time.Now().After(deadline) {
				fmt.Fprint(writer, "\r\033[K")
				return nil, fmt.Errorf("timeout waiting for Proxmox task %s", upid)
			}
		}
	}
}
