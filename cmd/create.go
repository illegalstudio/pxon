package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"pxon/internal/proxmox"
	"pxon/internal/sshkey"
	"pxon/internal/ui"
	"pxon/internal/wizard"
)

type createOptions struct {
	node         string
	vmid         int
	template     string
	image        string
	selectImage  bool
	rootfs       string
	storage      string
	diskSize     string
	password     string
	sshKeyPath   string
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
	Short: "Create a pxon-managed LXC container",
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

		image, err := selectCreateImage(cmd, client, node, cfg.DefaultImage)
		if err != nil {
			return err
		}
		if image.Kind == proxmox.ImageTemplate {
			return createFromContainerImage(cmd, client, cfg, image, vmid, strings.TrimSpace(args[0]))
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

		template := image.ID

		password := createOpts.password
		if password == "" {
			password = cfg.DefaultPassword
		}

		sshKeyPath := strings.TrimSpace(createOpts.sshKeyPath)
		if sshKeyPath == "" {
			sshKeyPath = strings.TrimSpace(cfg.DefaultSSHPublicKeyPath)
		}

		sshPublicKeys := ""
		if sshKeyPath != "" {
			sshPublicKeys, err = sshkey.ReadPublicKey(sshKeyPath)
			if err != nil {
				return err
			}
		}

		net0, err := resolveCreateNetwork(client, cfg, node)
		if err != nil {
			return err
		}

		req := proxmox.CreateContainerRequest{
			Node:          node,
			VMID:          vmid,
			Hostname:      strings.TrimSpace(args[0]),
			OSTemplate:    template,
			RootFS:        rootfs,
			Password:      password,
			SSHPublicKeys: sshPublicKeys,
			Memory:        createOpts.memory,
			Cores:         createOpts.cores,
			Swap:          createOpts.swap,
			Net0:          net0,
			Start:         createOpts.start,
			Unprivileged:  createOpts.unprivileged,
			Tags:          createOpts.tags,
		}

		upid, err := client.StartCreateContainer(req)
		if err != nil {
			return err
		}

		progressWriter := cmd.ErrOrStderr()
		if jsonEnabled() {
			progressWriter = io.Discard
		}

		taskStatus, err := waitForCreateTask(progressWriter, client, node, upid, req.Hostname, vmid, req.Start)
		if err != nil {
			return err
		}

		if jsonEnabled() {
			return ui.WriteJSON(cmd.OutOrStdout(), containerCreateResult{
				TaskStatus: taskStatus, Hostname: req.Hostname, VMID: vmid, Node: node,
				Image: image, RootFS: rootfs, Net0: net0, Stage: "completed",
			})
		}

		ui.RenderCreateSummary(cmd.OutOrStdout(), ui.CreateSummary{
			Hostname:   req.Hostname,
			VMID:       vmid,
			Node:       node,
			Status:     taskStatus.ExitStatus,
			Template:   req.OSTemplate,
			RootFS:     req.RootFS,
			Net0:       req.Net0,
			SSHKeyPath: sshKeyPath,
			UPID:       taskStatus.UPID,
		})
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVar(&createOpts.node, "node", "", "Proxmox node on which to create the container")
	createCmd.Flags().IntVar(&createOpts.vmid, "vmid", 0, "VMID to use; when omitted, request the next available ID")
	createCmd.Flags().StringVar(&createOpts.template, "template", "", "Image reference (alias for --image); when omitted, use the configured default")
	createCmd.Flags().StringVar(&createOpts.image, "image", "", "Image reference from images list (storage:vztmpl/... or lxc:<vmid>)")
	createCmd.Flags().BoolVar(&createOpts.selectImage, "select-image", false, "Choose an image interactively instead of using the configured default")
	createCmd.Flags().StringVar(&createOpts.rootfs, "rootfs", "", "Complete Proxmox rootfs value, for example storage:8")
	createCmd.Flags().StringVar(&createOpts.storage, "storage", "", "Storage for the rootfs disk; when omitted, use the configured default")
	createCmd.Flags().StringVar(&createOpts.diskSize, "disk-size", "", "Rootfs disk size; when omitted, use the configured default")
	createCmd.Flags().StringVar(&createOpts.password, "password", "", "Initial root password; when omitted, use the configured default")
	createCmd.Flags().StringVar(&createOpts.sshKeyPath, "ssh-key", "", "Path to the SSH public key to install; when omitted, use the configured default")
	createCmd.Flags().IntVar(&createOpts.memory, "memory", 512, "RAM in MB")
	createCmd.Flags().IntVar(&createOpts.cores, "cores", 1, "Number of CPU cores")
	createCmd.Flags().IntVar(&createOpts.swap, "swap", 512, "Swap in MB")
	createCmd.Flags().StringVar(&createOpts.net0, "net0", "", "Proxmox net0 network configuration; when omitted, use the configured default")
	createCmd.Flags().BoolVar(&createOpts.start, "start", true, "Start the container after creation; use --start=false to leave it stopped")
	createCmd.Flags().BoolVar(&createOpts.unprivileged, "unprivileged", true, "Create an unprivileged LXC container")
	createCmd.Flags().StringSliceVar(&createOpts.tags, "tag", nil, "Additional tag to apply; the pxon tag is always added")
}

func selectCreateImage(cmd *cobra.Command, client *proxmox.Client, node, configured string) (proxmox.Image, error) {
	if cmd.Flags().Changed("image") && cmd.Flags().Changed("template") {
		return proxmox.Image{}, fmt.Errorf("use only one of --image and --template")
	}
	for flag, value := range map[string]string{"image": createOpts.image, "template": createOpts.template} {
		if cmd.Flags().Changed(flag) && strings.TrimSpace(value) == "" {
			return proxmox.Image{}, fmt.Errorf("--%s requires a non-empty image reference", flag)
		}
	}
	if createOpts.selectImage {
		if jsonEnabled() || cmd.Flags().Changed("image") || cmd.Flags().Changed("template") {
			return proxmox.Image{}, fmt.Errorf("--select-image requires interactive output and cannot be combined with --image or --template; use images list --json and --image for automation")
		}
		images, err := client.ListImages(node)
		if err != nil {
			return proxmox.Image{}, err
		}
		if len(images) == 0 {
			return proxmox.Image{}, fmt.Errorf("no images available on node %q", node)
		}
		labels := make([]string, len(images))
		defaultIndex := 0
		for i, image := range images {
			labels[i] = fmt.Sprintf("%s (%s)", image.ID, image.Name)
			if image.ID == configured {
				defaultIndex = i
			}
		}
		selected, err := wizard.Select("Select the container image:", labels, defaultIndex)
		if err != nil {
			return proxmox.Image{}, err
		}
		for i, label := range labels {
			if label == selected {
				return images[i], nil
			}
		}
		return proxmox.Image{}, fmt.Errorf("no image selected")
	}
	reference := strings.TrimSpace(createOpts.image)
	if reference == "" {
		reference = strings.TrimSpace(createOpts.template)
	}
	if reference == "" {
		reference = strings.TrimSpace(configured)
	}
	if reference == "" {
		return proxmox.Image{}, fmt.Errorf("provide --image, use --select-image, or configure default_image; see pxon images list")
	}
	return client.ResolveImage(node, reference)
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
