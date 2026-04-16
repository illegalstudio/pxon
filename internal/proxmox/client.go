package proxmox

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"pxon/internal/config"
)

type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// ManagedTag is the Proxmox tag applied to LXC containers created by pxon.
const ManagedTag = "pxon"

type ClusterResourcesResponse struct {
	Data []Container `json:"data"`
}

type NodesResponse struct {
	Data []Node `json:"data"`
}

type StoragesResponse struct {
	Data []Storage `json:"data"`
}

type NetworksResponse struct {
	Data []NetworkInterface `json:"data"`
}

type Node struct {
	Node   string `json:"node"`
	Status string `json:"status"`
}

type Storage struct {
	Storage string `json:"storage"`
	Content string `json:"content"`
	Enabled int    `json:"enabled"`
	Active  int    `json:"active"`
}

type NetworkInterface struct {
	Iface  string `json:"iface"`
	Type   string `json:"type"`
	Active int    `json:"active"`
	Exists int    `json:"exists"`
}

type NextIDResponse struct {
	Data string `json:"data"`
}

type TaskUPIDResponse struct {
	Data string `json:"data"`
}

type TaskStatusResponse struct {
	Data TaskStatus `json:"data"`
}

type TaskLogResponse struct {
	Data []TaskLogEntry `json:"data"`
}

type TaskStatus struct {
	UPID       string `json:"upid"`
	Node       string `json:"node"`
	Type       string `json:"type"`
	ID         string `json:"id"`
	User       string `json:"user"`
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus,omitempty"`
	StartTime  int64  `json:"starttime"`
}

type TaskLogEntry struct {
	N int    `json:"n"`
	T string `json:"t"`
}

type StorageContentResponse struct {
	Data []StorageVolume `json:"data"`
}

type StorageVolume struct {
	VolID   string `json:"volid"`
	Content string `json:"content"`
}

type LXCConfigResponse struct {
	Data LXCConfig `json:"data"`
}

type LXCConfig struct {
	Net0 string `json:"net0"`
}

type Template struct {
	Storage string `json:"storage"`
	VolID   string `json:"volid"`
}

type CreateContainerRequest struct {
	Node         string
	VMID         int
	Hostname     string
	OSTemplate   string
	RootFS       string
	Password     string
	Memory       int
	Cores        int
	Swap         int
	Net0         string
	Start        bool
	Unprivileged bool
	Tags         []string
}

type Container struct {
	ID        string  `json:"id"`
	CPU       float64 `json:"cpu"`
	Uptime    int64   `json:"uptime"`
	NetIn     int64   `json:"netin"`
	NetOut    int64   `json:"netout"`
	Node      string  `json:"node"`
	Template  int     `json:"template"`
	Mem       int64   `json:"mem"`
	Status    string  `json:"status"`
	DiskWrite int64   `json:"diskwrite"`
	Disk      int64   `json:"disk"`
	VMID      int     `json:"vmid"`
	MaxMem    int64   `json:"maxmem"`
	MaxDisk   int64   `json:"maxdisk"`
	Type      string  `json:"type"`
	DiskRead  int64   `json:"diskread"`
	MaxCPU    int     `json:"maxcpu"`
	Name      string  `json:"name"`
	Tags      string  `json:"tags,omitempty"`
	Managed   bool    `json:"managed"`
}

func NewClient(cfg *config.Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("missing configuration")
	}

	endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if endpoint == "" {
		return nil, fmt.Errorf("missing Proxmox endpoint")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &Client{
		endpoint: endpoint,
		token:    fmt.Sprintf("PVEAPIToken=%s=%s", cfg.TokenID, cfg.TokenSecret),
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}, nil
}

func (c *Client) ListContainers() ([]byte, error) {
	req, err := c.newRequest(http.MethodGet, c.endpoint+"/cluster/resources?type=vm", nil)
	if err != nil {
		return nil, fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var resources ClusterResourcesResponse
	if err := json.Unmarshal(body, &resources); err != nil {
		return nil, fmt.Errorf("decode Proxmox response: %w", err)
	}

	filtered := ClusterResourcesResponse{
		Data: make([]Container, 0, len(resources.Data)),
	}

	for _, container := range resources.Data {
		if container.Type != "lxc" {
			continue
		}

		container.Managed = HasManagedTag(container.Tags)
		filtered.Data = append(filtered.Data, container)
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		return nil, fmt.Errorf("encode filtered containers: %w", err)
	}

	return data, nil
}

func (c *Client) DefaultNode() (string, error) {
	req, err := c.newRequest(http.MethodGet, c.endpoint+"/nodes", nil)
	if err != nil {
		return "", fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var nodes NodesResponse
	if err := json.Unmarshal(body, &nodes); err != nil {
		return "", fmt.Errorf("decode Proxmox response: %w", err)
	}

	if len(nodes.Data) == 0 {
		return "", fmt.Errorf("no Proxmox nodes available")
	}

	if len(nodes.Data) == 1 {
		return nodes.Data[0].Node, nil
	}

	for _, node := range nodes.Data {
		if node.Status == "online" {
			return node.Node, nil
		}
	}

	return "", fmt.Errorf("multiple Proxmox nodes available; specify --node")
}

func (c *Client) ListStorages(node string) ([]Storage, error) {
	req, err := c.newRequest(
		http.MethodGet,
		fmt.Sprintf("%s/nodes/%s/storage", c.endpoint, url.PathEscape(node)),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var storagesResp StoragesResponse
	if err := json.Unmarshal(body, &storagesResp); err != nil {
		return nil, fmt.Errorf("decode Proxmox response: %w", err)
	}

	storages := make([]Storage, 0, len(storagesResp.Data))
	for _, storage := range storagesResp.Data {
		if storage.Enabled != 1 || storage.Active != 1 {
			continue
		}

		if !hasStorageContent(storage.Content, "rootdir") {
			continue
		}

		storages = append(storages, storage)
	}

	return storages, nil
}

func (c *Client) ListBridges(node string) ([]string, error) {
	req, err := c.newRequest(
		http.MethodGet,
		fmt.Sprintf("%s/nodes/%s/network", c.endpoint, url.PathEscape(node)),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var networksResp NetworksResponse
	if err := json.Unmarshal(body, &networksResp); err != nil {
		return nil, fmt.Errorf("decode Proxmox response: %w", err)
	}

	bridges := make([]string, 0)
	for _, network := range networksResp.Data {
		if network.Type != "bridge" {
			continue
		}

		bridges = append(bridges, network.Iface)
	}

	return bridges, nil
}

func (c *Client) ListTemplates(node string) ([]Template, error) {
	req, err := c.newRequest(
		http.MethodGet,
		fmt.Sprintf("%s/nodes/%s/storage", c.endpoint, url.PathEscape(node)),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var storagesResp StoragesResponse
	if err := json.Unmarshal(body, &storagesResp); err != nil {
		return nil, fmt.Errorf("decode Proxmox response: %w", err)
	}

	templates := make([]Template, 0)
	for _, storage := range storagesResp.Data {
		if storage.Enabled != 1 || storage.Active != 1 {
			continue
		}

		if !hasStorageContent(storage.Content, "vztmpl") {
			continue
		}

		content, err := c.StorageContent(node, storage.Storage)
		if err != nil {
			return nil, err
		}

		for _, volume := range content {
			if volume.Content != "vztmpl" {
				continue
			}

			templates = append(templates, Template{
				Storage: storage.Storage,
				VolID:   volume.VolID,
			})
		}
	}

	return templates, nil
}

func (c *Client) NextID() (int, error) {
	req, err := c.newRequest(http.MethodGet, c.endpoint+"/cluster/nextid", nil)
	if err != nil {
		return 0, fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var nextID NextIDResponse
	if err := json.Unmarshal(body, &nextID); err != nil {
		return 0, fmt.Errorf("decode Proxmox response: %w", err)
	}

	vmid, err := strconv.Atoi(strings.TrimSpace(nextID.Data))
	if err != nil {
		return 0, fmt.Errorf("invalid next VMID %q: %w", nextID.Data, err)
	}

	return vmid, nil
}

func (c *Client) ContainersByNode(node string) ([]Container, error) {
	req, err := c.newRequest(http.MethodGet, c.endpoint+"/cluster/resources?type=vm", nil)
	if err != nil {
		return nil, fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var resources ClusterResourcesResponse
	if err := json.Unmarshal(body, &resources); err != nil {
		return nil, fmt.Errorf("decode Proxmox response: %w", err)
	}

	containers := make([]Container, 0, len(resources.Data))
	for _, container := range resources.Data {
		if container.Type != "lxc" || container.Node != node {
			continue
		}

		containers = append(containers, container)
	}

	return containers, nil
}

func (c *Client) ContainerConfig(node string, vmid int) (*LXCConfig, error) {
	req, err := c.newRequest(
		http.MethodGet,
		fmt.Sprintf("%s/nodes/%s/lxc/%d/config", c.endpoint, url.PathEscape(node), vmid),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var cfgResp LXCConfigResponse
	if err := json.Unmarshal(body, &cfgResp); err != nil {
		return nil, fmt.Errorf("decode Proxmox response: %w", err)
	}

	return &cfgResp.Data, nil
}

func (c *Client) UsedIPv4Addresses(node string) (map[string]struct{}, error) {
	containers, err := c.ContainersByNode(node)
	if err != nil {
		return nil, err
	}

	used := make(map[string]struct{}, len(containers))
	for _, container := range containers {
		cfg, err := c.ContainerConfig(node, container.VMID)
		if err != nil {
			return nil, err
		}

		ip, ok := IPv4FromNet0(cfg.Net0)
		if !ok {
			continue
		}

		used[ip] = struct{}{}
	}

	return used, nil
}

func (c *Client) CreateContainer(createReq CreateContainerRequest) ([]byte, error) {
	upid, err := c.StartCreateContainer(createReq)
	if err != nil {
		return nil, err
	}

	taskStatus, err := c.WaitForTask(createReq.Node, upid, 2*time.Minute)
	if err != nil {
		return nil, err
	}

	if taskStatus.ExitStatus != "OK" {
		return nil, fmt.Errorf("Proxmox task failed: %s", taskStatus.ExitStatus)
	}

	result, err := json.Marshal(taskStatus)
	if err != nil {
		return nil, fmt.Errorf("encode Proxmox task result: %w", err)
	}

	return result, nil
}

func (c *Client) StartCreateContainer(createReq CreateContainerRequest) (string, error) {
	if err := createReq.Validate(); err != nil {
		return "", err
	}

	if err := c.ValidateTemplateExists(createReq.Node, createReq.OSTemplate); err != nil {
		return "", err
	}

	values := url.Values{}
	values.Set("vmid", strconv.Itoa(createReq.VMID))
	values.Set("hostname", createReq.Hostname)
	values.Set("ostemplate", createReq.OSTemplate)
	values.Set("rootfs", createReq.RootFS)
	values.Set("memory", strconv.Itoa(createReq.Memory))
	values.Set("cores", strconv.Itoa(createReq.Cores))
	values.Set("swap", strconv.Itoa(createReq.Swap))
	values.Set("unprivileged", boolToProxmox(createReq.Unprivileged))
	values.Set("tags", JoinManagedTags(createReq.Tags))

	if createReq.Password != "" {
		values.Set("password", createReq.Password)
	}

	if createReq.Net0 != "" {
		values.Set("net0", createReq.Net0)
	}

	if createReq.Start {
		values.Set("start", "1")
	}

	req, err := c.newRequest(
		http.MethodPost,
		fmt.Sprintf("%s/nodes/%s/lxc", c.endpoint, url.PathEscape(createReq.Node)),
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("build Proxmox request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var upidResp TaskUPIDResponse
	if err := json.Unmarshal(body, &upidResp); err != nil {
		return "", fmt.Errorf("decode Proxmox response: %w", err)
	}

	return upidResp.Data, nil
}

func HasManagedTag(tags string) bool {
	for _, tag := range strings.Split(tags, ";") {
		if strings.TrimSpace(tag) == ManagedTag {
			return true
		}
	}

	return false
}

func JoinManagedTags(tags []string) string {
	seen := map[string]struct{}{
		ManagedTag: {},
	}
	normalized := []string{ManagedTag}

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		if _, ok := seen[tag]; ok {
			continue
		}

		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}

	return strings.Join(normalized, ";")
}

func BuildRootFS(storage, diskSize string) (string, error) {
	storage = strings.TrimSpace(storage)
	diskSize = strings.TrimSpace(diskSize)

	switch {
	case storage == "" && diskSize == "":
		return "", fmt.Errorf("missing rootfs configuration: provide --rootfs or both --storage and --disk-size")
	case storage == "" || diskSize == "":
		return "", fmt.Errorf("incomplete rootfs configuration: --storage and --disk-size must be used together")
	default:
		return storage + ":" + diskSize, nil
	}
}

func (r CreateContainerRequest) Validate() error {
	switch {
	case strings.TrimSpace(r.Node) == "":
		return fmt.Errorf("missing Proxmox node")
	case r.VMID <= 0:
		return fmt.Errorf("invalid VMID %d", r.VMID)
	case strings.TrimSpace(r.Hostname) == "":
		return fmt.Errorf("missing container hostname")
	case strings.TrimSpace(r.OSTemplate) == "":
		return fmt.Errorf("missing container template")
	case strings.TrimSpace(r.RootFS) == "":
		return fmt.Errorf("missing container rootfs")
	case r.Memory <= 0:
		return fmt.Errorf("memory must be greater than zero")
	case r.Cores <= 0:
		return fmt.Errorf("cores must be greater than zero")
	case r.Swap < 0:
		return fmt.Errorf("swap cannot be negative")
	default:
		return nil
	}
}

func (c *Client) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.token)
	return req, nil
}

func (c *Client) WaitForTask(node, upid string, timeout time.Duration) (*TaskStatus, error) {
	if strings.TrimSpace(node) == "" {
		return nil, fmt.Errorf("missing Proxmox node for task polling")
	}

	if strings.TrimSpace(upid) == "" {
		return nil, fmt.Errorf("missing Proxmox task UPID")
	}

	deadline := time.Now().Add(timeout)

	for {
		taskStatus, err := c.TaskStatus(node, upid)
		if err != nil {
			return nil, err
		}

		if taskStatus.Status == "stopped" {
			return taskStatus, nil
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for Proxmox task %s", upid)
		}

		time.Sleep(2 * time.Second)
	}
}

func (c *Client) TaskStatus(node, upid string) (*TaskStatus, error) {
	req, err := c.newRequest(
		http.MethodGet,
		fmt.Sprintf("%s/nodes/%s/tasks/%s/status", c.endpoint, url.PathEscape(node), url.PathEscape(upid)),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var taskStatusResp TaskStatusResponse
	if err := json.Unmarshal(body, &taskStatusResp); err != nil {
		return nil, fmt.Errorf("decode Proxmox response: %w", err)
	}

	return &taskStatusResp.Data, nil
}

func (c *Client) TaskLog(node, upid string) ([]TaskLogEntry, error) {
	req, err := c.newRequest(
		http.MethodGet,
		fmt.Sprintf("%s/nodes/%s/tasks/%s/log", c.endpoint, url.PathEscape(node), url.PathEscape(upid)),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var taskLogResp TaskLogResponse
	if err := json.Unmarshal(body, &taskLogResp); err != nil {
		return nil, fmt.Errorf("decode Proxmox response: %w", err)
	}

	return taskLogResp.Data, nil
}

func (c *Client) ValidateTemplateExists(node, ostemplate string) error {
	storage, _, ok := strings.Cut(strings.TrimSpace(ostemplate), ":")
	if !ok || storage == "" {
		return fmt.Errorf("invalid template reference %q: expected storage:vztmpl/...", ostemplate)
	}

	content, err := c.StorageContent(node, storage)
	if err != nil {
		return err
	}

	for _, volume := range content {
		if volume.Content == "vztmpl" && volume.VolID == ostemplate {
			return nil
		}
	}

	return fmt.Errorf("template %q not found on storage %q", ostemplate, storage)
}

func (c *Client) StorageContent(node, storage string) ([]StorageVolume, error) {
	req, err := c.newRequest(
		http.MethodGet,
		fmt.Sprintf("%s/nodes/%s/storage/%s/content", c.endpoint, url.PathEscape(node), url.PathEscape(storage)),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build Proxmox request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Proxmox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Proxmox response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Proxmox API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var content StorageContentResponse
	if err := json.Unmarshal(body, &content); err != nil {
		return nil, fmt.Errorf("decode Proxmox response: %w", err)
	}

	return content.Data, nil
}

func boolToProxmox(value bool) string {
	if value {
		return "1"
	}

	return "0"
}

func hasStorageContent(contents, target string) bool {
	for _, content := range strings.Split(contents, ",") {
		if strings.TrimSpace(content) == target {
			return true
		}
	}

	return false
}

func IPv4FromNet0(net0 string) (string, bool) {
	for _, part := range strings.Split(net0, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || key != "ip" {
			continue
		}

		value = strings.TrimSpace(value)
		if value == "" || value == "dhcp" || value == "manual" {
			return "", false
		}

		ip, _, _ := strings.Cut(value, "/")
		if ip == "" {
			return "", false
		}

		return ip, true
	}

	return "", false
}
