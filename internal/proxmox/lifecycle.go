package proxmox

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ContainerState struct {
	Status string `json:"status"`
	Uptime int64  `json:"uptime"`
}

func validateContainerTarget(node string, vmid int) error {
	if strings.TrimSpace(node) == "" {
		return fmt.Errorf("missing Proxmox node")
	}
	if vmid < 100 || vmid > 999999999 {
		return fmt.Errorf("invalid VMID %d", vmid)
	}
	return nil
}

func (c *Client) ContainerState(node string, vmid int) (*ContainerState, error) {
	if err := validateContainerTarget(node, vmid); err != nil {
		return nil, err
	}
	var response struct {
		Data ContainerState `json:"data"`
	}
	if err := c.imageRequest(http.MethodGet, containerPath(node, vmid)+"/status/current", nil, &response); err != nil {
		return nil, err
	}
	if strings.TrimSpace(response.Data.Status) == "" {
		return nil, fmt.Errorf("Proxmox returned an empty container status")
	}
	return &response.Data, nil
}

// ShutdownContainer requests a clean shutdown without a forced-stop fallback.
func (c *Client) ShutdownContainer(node string, vmid int, timeout time.Duration) (string, error) {
	if err := validateContainerTarget(node, vmid); err != nil {
		return "", err
	}
	if timeout <= 0 {
		return "", fmt.Errorf("shutdown timeout must be positive")
	}
	values := url.Values{
		"timeout":   {strconv.FormatInt(int64(math.Ceil(timeout.Seconds())), 10)},
		"forceStop": {"0"},
	}
	var response TaskUPIDResponse
	if err := c.imageRequest(http.MethodPost, containerPath(node, vmid)+"/status/shutdown", values, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Data) == "" {
		return "", fmt.Errorf("Proxmox shutdown returned an empty task UPID")
	}
	return response.Data, nil
}
