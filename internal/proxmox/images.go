package proxmox

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
)

const (
	ImageArchive  = "archive"
	ImageTemplate = "template"
)

// Image.ID can be passed unchanged to create --image or default_image.
type Image struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Node    string `json:"node"`
	Storage string `json:"storage,omitempty"`
	VMID    int    `json:"vmid,omitempty"`
}

func (cfg *LXCConfig) UnmarshalJSON(data []byte) error {
	type plain LXCConfig
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for key := range fields {
		if index, err := strconv.Atoi(strings.TrimPrefix(key, "net")); strings.HasPrefix(key, "net") && err == nil && index >= 0 {
			value.NetworkKeys = append(value.NetworkKeys, key)
		}
	}
	sort.Strings(value.NetworkKeys)
	*cfg = LXCConfig(value)
	return nil
}

func (c *Client) ListImages(node string) ([]Image, error) {
	archives, err := c.ListTemplates(node)
	if err != nil {
		return nil, err
	}
	containers, err := c.ContainersByNode(node)
	if err != nil {
		return nil, err
	}
	images := make([]Image, 0, len(archives)+len(containers))
	for _, archive := range archives {
		images = append(images, Image{ID: archive.VolID, Kind: ImageArchive, Name: path.Base(archive.VolID), Node: node, Storage: archive.Storage})
	}
	for _, container := range containers {
		if container.Template == 1 && HasManagedTag(container.Tags) {
			images = append(images, imageFromContainer(container))
		}
	}
	sort.Slice(images, func(i, j int) bool { return images[i].ID < images[j].ID })
	return images, nil
}

func imageFromContainer(container Container) Image {
	return Image{ID: fmt.Sprintf("lxc:%d", container.VMID), Kind: ImageTemplate, Name: container.Name, Node: container.Node, VMID: container.VMID}
}

func (c *Client) ResolveImage(node, reference string) (Image, error) {
	reference = strings.TrimSpace(reference)
	if strings.HasPrefix(reference, "lxc:") && !strings.HasPrefix(reference, "lxc:vztmpl/") {
		vmid, err := strconv.Atoi(strings.TrimPrefix(reference, "lxc:"))
		if err != nil || vmid < 100 || vmid > 999999999 {
			return Image{}, fmt.Errorf("invalid image reference %q: expected lxc:<template-vmid>", reference)
		}
		containers, err := c.ContainersByNode(node)
		if err != nil {
			return Image{}, err
		}
		for _, container := range containers {
			if container.VMID == vmid && container.Template == 1 && HasManagedTag(container.Tags) {
				return imageFromContainer(container), nil
			}
		}
		return Image{}, fmt.Errorf("managed template %q not found on node %q; run pxon images list --node %s", reference, node, node)
	}
	if err := c.ValidateTemplateExists(node, reference); err != nil {
		return Image{}, err
	}
	storage, _, _ := strings.Cut(reference, ":")
	return Image{ID: reference, Kind: ImageArchive, Name: path.Base(reference), Node: node, Storage: storage}, nil
}

type CloneContainerRequest struct {
	Node       string
	SourceVMID int
	VMID       int
	Hostname   string
	Storage    string
	Snapshot   string
}

func (r CloneContainerRequest) Validate() error {
	if strings.TrimSpace(r.Node) == "" || strings.TrimSpace(r.Hostname) == "" {
		return fmt.Errorf("clone requires a node and hostname")
	}
	if r.SourceVMID < 100 || r.SourceVMID > 999999999 || r.VMID < 100 || r.VMID > 999999999 || r.SourceVMID == r.VMID {
		return fmt.Errorf("clone requires distinct source and destination VMIDs between 100 and 999999999")
	}
	return nil
}

func (c *Client) StartCloneContainer(request CloneContainerRequest) (string, error) {
	if err := request.Validate(); err != nil {
		return "", err
	}
	values := url.Values{"newid": {strconv.Itoa(request.VMID)}, "hostname": {request.Hostname}, "full": {"1"}}
	if request.Storage != "" {
		values.Set("storage", request.Storage)
	}
	if request.Snapshot != "" {
		values.Set("snapname", request.Snapshot)
	}
	var response TaskUPIDResponse
	if err := c.imageRequest(http.MethodPost, containerPath(request.Node, request.SourceVMID)+"/clone", values, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Data) == "" {
		return "", fmt.Errorf("Proxmox clone returned an empty task UPID")
	}
	return response.Data, nil
}

func (c *Client) UpdateContainer(node string, vmid int, values url.Values) error {
	return c.imageRequest(http.MethodPut, containerPath(node, vmid)+"/config", values, nil)
}

// ConvertToTemplate is synchronous: the Proxmox API returns null, not a UPID.
func (c *Client) ConvertToTemplate(node string, vmid int) error {
	return c.imageRequest(http.MethodPost, containerPath(node, vmid)+"/template", nil, nil)
}

func (c *Client) StartContainer(node string, vmid int) (string, error) {
	if err := validateContainerTarget(node, vmid); err != nil {
		return "", err
	}
	var response TaskUPIDResponse
	if err := c.imageRequest(http.MethodPost, containerPath(node, vmid)+"/status/start", nil, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Data) == "" {
		return "", fmt.Errorf("Proxmox start returned an empty task UPID")
	}
	return response.Data, nil
}

func containerPath(node string, vmid int) string {
	return fmt.Sprintf("/nodes/%s/lxc/%d", url.PathEscape(node), vmid)
}

func (c *Client) imageRequest(method, path string, values url.Values, result any) error {
	req, err := c.newRequest(method, c.endpoint+path, strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("build Proxmox request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request Proxmox API: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read Proxmox response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Proxmox API returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decode Proxmox response: %w", err)
		}
	}
	return nil
}
