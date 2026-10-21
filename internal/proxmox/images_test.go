package proxmox

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestListImagesScopesTemplatesAndArchives(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nodes/pve/storage/local/content":
			if r.URL.Query().Get("content") != "vztmpl" {
				t.Errorf("content query = %q, want vztmpl", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"data":[{"volid":"local:vztmpl/ubuntu.tar.zst","content":"vztmpl"},{"volid":"local:backup/a.tar.zst","content":"backup"}]}`)
		case "/cluster/resources":
			if r.URL.Query().Get("type") == "storage" {
				fmt.Fprint(w, `{"data":[{"node":"pve","type":"storage","storage":"local","status":"available","content":"backup,vztmpl"},{"node":"pve","type":"storage","storage":"offline-nfs","status":"unknown","content":"vztmpl"},{"node":"other","type":"storage","storage":"other-node","status":"available","content":"vztmpl"},{"node":"pve","type":"storage","storage":"rootfs-only","status":"available","content":"images,rootdir"}]}`)
				return
			}
			fmt.Fprint(w, `{"data":[{"vmid":101,"name":"custom","node":"pve","type":"lxc","template":1,"tags":"custom;pxon"},{"vmid":102,"node":"pve","type":"lxc","template":1,"tags":"not-pxon"},{"vmid":103,"node":"pve","type":"lxc","tags":"pxon"},{"vmid":104,"node":"other","type":"lxc","template":1,"tags":"pxon"},{"vmid":105,"node":"pve","type":"qemu","template":1,"tags":"pxon"}]}`)
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := &Client{endpoint: server.URL, httpClient: server.Client()}
	images, err := client.ListImages("pve")
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 2 || images[0].ID != "local:vztmpl/ubuntu.tar.zst" || images[1].ID != "lxc:101" {
		t.Fatalf("images = %#v", images)
	}
	if images[0].Kind != ImageArchive || images[1].Kind != ImageTemplate {
		t.Fatalf("image kinds = %#v", images)
	}
	for _, ref := range []string{"lxc:102", "lxc:103", "lxc:104", "lxc:105", "lxc:0", "lxc:invalid"} {
		if _, err := client.ResolveImage("pve", ref); err == nil {
			t.Errorf("resolved invalid or out-of-scope image %q", ref)
		}
	}
	if image, err := client.ResolveImage("pve", "lxc:101"); err != nil || image.Name != "custom" {
		t.Fatalf("resolved image = %#v, error = %v", image, err)
	}
}

func TestListTemplatesSkipsOfflineStorageWithoutProbingMounts(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		if r.URL.RequestURI() != "/cluster/resources?type=storage" {
			t.Errorf("unexpected storage probe: %s", r.URL)
			http.Error(w, "offline storage must not be probed", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"data":[{"node":"pve","type":"storage","storage":"offline-nfs","status":"unknown","content":"vztmpl"}]}`)
	}))
	defer server.Close()
	client := &Client{endpoint: server.URL, httpClient: server.Client()}
	templates, err := client.ListTemplates("pve")
	if err != nil || templates == nil || len(templates) != 0 || len(requests) != 1 {
		t.Fatalf("templates = %#v, error = %v, requests = %v", templates, err, requests)
	}
}

func TestValidateTemplateFiltersStorageContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/nodes/pve/storage/local/content?content=vztmpl" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		fmt.Fprint(w, `{"data":[{"volid":"local:vztmpl/ubuntu.tar.zst","content":"vztmpl"}]}`)
	}))
	defer server.Close()
	client := &Client{endpoint: server.URL, httpClient: server.Client()}
	if err := client.ValidateTemplateExists("pve", "local:vztmpl/ubuntu.tar.zst"); err != nil {
		t.Fatal(err)
	}
}

func TestCloneUsesFullCopyAndSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/nodes/node%201/lxc/101/clone" {
			t.Errorf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{"newid": "201", "hostname": "app-image", "full": "1", "storage": "local-lvm", "snapname": "ready"} {
			if got := r.Form.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		fmt.Fprint(w, `{"data":"UPID:clone"}`)
	}))
	defer server.Close()
	client := &Client{endpoint: server.URL, httpClient: server.Client()}
	upid, err := client.StartCloneContainer(CloneContainerRequest{Node: "node 1", SourceVMID: 101, VMID: 201, Hostname: "app-image", Storage: "local-lvm", Snapshot: "ready"})
	if err != nil || upid != "UPID:clone" {
		t.Fatalf("UPID = %q, error = %v", upid, err)
	}
}

func TestCloneRejectsInvalidTargetsBeforeRequests(t *testing.T) {
	for _, request := range []CloneContainerRequest{
		{Node: "pve", SourceVMID: 101, VMID: 101, Hostname: "same"},
		{Node: "pve", SourceVMID: 101, VMID: 99, Hostname: "invalid"},
		{Node: "pve", SourceVMID: 1, VMID: 201, Hostname: "invalid"},
		{SourceVMID: 101, VMID: 201, Hostname: "no-node"},
		{Node: "pve", SourceVMID: 101, VMID: 201},
	} {
		if _, err := (&Client{}).StartCloneContainer(request); err == nil {
			t.Errorf("accepted invalid clone: %#v", request)
		}
	}
}

func TestConvertTemplateAcceptsSynchronousNullResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/nodes/pve/lxc/201/template" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer server.Close()
	if err := (&Client{endpoint: server.URL, httpClient: server.Client()}).ConvertToTemplate("pve", 201); err != nil {
		t.Fatal(err)
	}
}

func TestLXCConfigFindsAllNetworkInterfaces(t *testing.T) {
	var cfg LXCConfig
	if err := json.Unmarshal([]byte(`{"net0":"name=eth0,ip=192.0.2.1/24","net10":"name=eth10","net1":"name=eth1","network":"unrelated","rootfs":"local:vm-201-disk-0,size=16G","template":1}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.NetworkKeys, []string{"net0", "net1", "net10"}) || cfg.Template != 1 || cfg.RootFS == "" {
		t.Fatalf("config = %#v", cfg)
	}
}
