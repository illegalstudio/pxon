package proxmox

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	req, err := http.NewRequest(http.MethodGet, c.endpoint+"/cluster/resources?type=vm", nil)
	if err != nil {
		return nil, fmt.Errorf("build Proxmox request: %w", err)
	}

	req.Header.Set("Authorization", c.token)

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

func HasManagedTag(tags string) bool {
	for _, tag := range strings.Split(tags, ";") {
		if strings.TrimSpace(tag) == ManagedTag {
			return true
		}
	}

	return false
}
