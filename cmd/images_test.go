package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

type imageAPIFixture struct {
	running   bool
	template  bool
	fail      string
	mutations []string
	clone     url.Values
	update    url.Values
	create    url.Values
}

func (f *imageAPIFixture) serve(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodGet {
		f.mutations = append(f.mutations, r.Method+" "+r.URL.Path)
	}
	status, template := "stopped", 0
	if f.running {
		status = "running"
	}
	if f.template {
		template = 1
	}
	switch {
	case r.URL.Path == "/nodes":
		fmt.Fprint(w, `{"data":[{"node":"pve","status":"online"}]}`)
	case r.URL.Path == "/cluster/nextid":
		fmt.Fprint(w, `{"data":"201"}`)
	case r.URL.Path == "/cluster/resources":
		if r.URL.Query().Get("type") == "storage" {
			fmt.Fprint(w, `{"data":[{"node":"pve","type":"storage","storage":"local","status":"available","content":"vztmpl"}]}`)
			return
		}
		fmt.Fprintf(w, `{"data":[{"vmid":101,"name":"configured","node":"pve","type":"lxc","status":%q,"template":%d,"tags":"pxon;app"}]}`, status, template)
	case r.URL.Path == "/nodes/pve/storage/local/content":
		fmt.Fprint(w, `{"data":[{"volid":"local:vztmpl/ubuntu.tar.zst","content":"vztmpl"}]}`)
	case r.URL.Path == "/nodes/pve/lxc/101/clone":
		_ = r.ParseForm()
		f.clone = r.PostForm
		fmt.Fprint(w, `{"data":"UPID:clone"}`)
	case r.URL.Path == "/nodes/pve/lxc" && r.Method == http.MethodPost:
		_ = r.ParseForm()
		f.create = r.PostForm
		fmt.Fprint(w, `{"data":"UPID:clone"}`)
	case strings.HasSuffix(r.URL.Path, "/log") && strings.Contains(r.URL.Path, "/tasks/"):
		fmt.Fprint(w, `{"data":[]}`)
	case strings.HasSuffix(r.URL.Path, "/config") && r.Method == http.MethodGet:
		fmt.Fprint(w, `{"data":{"net0":"name=eth0,ip=192.0.2.10/24","net1":"name=eth1,ip=192.0.2.11/24","rootfs":"local-lvm:vm-201-disk-0,size=16G"}}`)
	case r.URL.Path == "/nodes/pve/lxc/201/config" && r.Method == http.MethodPut:
		_ = r.ParseForm()
		f.update = r.PostForm
		if f.fail == "configure" {
			http.Error(w, "configuration failed", 500)
		} else {
			fmt.Fprint(w, `{"data":null}`)
		}
	case r.URL.Path == "/nodes/pve/lxc/201/template":
		if f.fail == "template" {
			http.Error(w, "conversion failed", 500)
		} else {
			fmt.Fprint(w, `{"data":null}`)
		}
	case r.URL.Path == "/nodes/pve/lxc/201/status/start":
		fmt.Fprint(w, `{"data":"UPID:start"}`)
	case strings.HasSuffix(r.URL.Path, "/status") && strings.Contains(r.URL.Path, "/tasks/"):
		exit := "OK"
		if f.fail == "clone" {
			exit = "copy failed"
		}
		upid := "UPID:clone"
		if strings.Contains(r.URL.Path, "UPID:start") {
			upid = "UPID:start"
		}
		fmt.Fprintf(w, `{"data":{"upid":%q,"node":"pve","status":"stopped","exitstatus":%q}}`, upid, exit)
	default:
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		http.NotFound(w, r)
	}
}

func fixtureServer(t *testing.T, fixture *imageAPIFixture) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fixture.serve(t, w, r) }))
	t.Cleanup(server.Close)
	cliTestEnvironment(t, server.URL)
}

func TestImagesCreatePreservesSourceAndIsolatesTemplate(t *testing.T) {
	fixture := &imageAPIFixture{}
	fixtureServer(t, fixture)
	result, stderr, err := runTestCLI(t, "images", "create", "configured", "app-image", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var image map[string]any
	_ = json.Unmarshal(result["image"], &image)
	if image["id"] != "lxc:201" || string(result["stage"]) != `"completed"` || stderr != "" {
		t.Fatalf("result = %s, stderr = %q", result, stderr)
	}
	want := []string{"POST /nodes/pve/lxc/101/clone", "PUT /nodes/pve/lxc/201/config", "POST /nodes/pve/lxc/201/template"}
	if !reflect.DeepEqual(fixture.mutations, want) {
		t.Fatalf("mutations = %v, want %v", fixture.mutations, want)
	}
	if fixture.clone.Get("full") != "1" || fixture.update.Get("delete") != "net0,net1" || fixture.update.Get("onboot") != "0" || fixture.update.Get("tags") != "pxon;pxon-image" {
		t.Fatalf("clone = %v, update = %v", fixture.clone, fixture.update)
	}
}

func TestImagesCreateRunningSourceRequiresSnapshot(t *testing.T) {
	fixture := &imageAPIFixture{running: true}
	fixtureServer(t, fixture)
	_, _, err := runTestCLI(t, "images", "create", "101", "app-image", "--json")
	if err == nil || len(fixture.mutations) != 0 {
		t.Fatalf("error = %v, mutations = %v", err, fixture.mutations)
	}
	_, _, err = runTestCLI(t, "images", "create", "101", "app-image", "--snapshot", "ready", "--json")
	if err != nil || fixture.clone.Get("snapname") != "ready" {
		t.Fatalf("error = %v, clone = %v", err, fixture.clone)
	}
}

func TestImagesCreateReportsPartialFailuresWithoutCleanup(t *testing.T) {
	for _, stage := range []string{"clone", "configure", "template"} {
		t.Run(stage, func(t *testing.T) {
			fixture := &imageAPIFixture{fail: stage}
			fixtureServer(t, fixture)
			result, _, err := runTestCLI(t, "images", "create", "101", "app-image", "--json")
			if err == nil || string(result["stage"]) != fmt.Sprintf("%q", stage) || result["error"] == nil || result["image"] == nil || result["upid"] == nil {
				t.Fatalf("error = %v, result = %s", err, result)
			}
			for _, mutation := range fixture.mutations {
				if strings.HasPrefix(mutation, "DELETE") || strings.Contains(mutation, "/status/") {
					t.Fatalf("unexpected cleanup or source state change: %s", mutation)
				}
			}
		})
	}
}

func TestCreateFromTemplateSetsNewNetworkAndInheritsDiskAndResources(t *testing.T) {
	fixture := &imageAPIFixture{template: true}
	fixtureServer(t, fixture)
	result, stderr, err := runTestCLI(t, "create", "app-02", "--image", "lxc:101", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if string(result["stage"]) != `"completed"` || result["clone_task"] == nil || result["start_task"] == nil || stderr != "" {
		t.Fatalf("result = %s, stderr = %q", result, stderr)
	}
	if fixture.update.Get("delete") != "net1" || fixture.update.Get("net0") == "" || fixture.update.Get("tags") != "pxon" || fixture.update.Has("memory") || fixture.update.Has("rootfs") || fixture.update.Has("ssh-public-keys") {
		t.Fatalf("update = %v", fixture.update)
	}
	if !strings.HasSuffix(fixture.mutations[len(fixture.mutations)-1], "/201/status/start") {
		t.Fatalf("container not started after configuring: %v", fixture.mutations)
	}
}

func TestCreateFromTemplateSupportsOverridesAndStoppedClone(t *testing.T) {
	fixture := &imageAPIFixture{template: true}
	fixtureServer(t, fixture)
	_, _, err := runTestCLI(t, "create", "app-02", "--image", "lxc:101", "--memory", "2048", "--start=false", "--json")
	if err != nil || fixture.update.Get("memory") != "2048" || len(fixture.mutations) != 2 {
		t.Fatalf("error = %v, update = %v, mutations = %v", err, fixture.update, fixture.mutations)
	}
}

func TestCreateFromTemplateRejectsUnsupportedFlagsBeforeCloning(t *testing.T) {
	for _, args := range [][]string{{"--ssh-key", "missing.pub"}, {"--password", "test-password"}, {"--disk-size", "16"}, {"--rootfs", "local:8"}, {"--unprivileged=false"}, {"--vmid", "101"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			fixture := &imageAPIFixture{template: true}
			fixtureServer(t, fixture)
			invocation := append([]string{"create", "app-02", "--image", "lxc:101", "--json"}, args...)
			_, _, err := runTestCLI(t, invocation...)
			if err == nil || len(fixture.mutations) != 0 {
				t.Fatalf("error = %v, mutations = %v", err, fixture.mutations)
			}
		})
	}
}

func TestCreateFromTemplateReportsConfigurationFailureAndDoesNotStart(t *testing.T) {
	fixture := &imageAPIFixture{template: true, fail: "configure"}
	fixtureServer(t, fixture)
	result, _, err := runTestCLI(t, "create", "app-02", "--image", "lxc:101", "--json")
	if err == nil || string(result["stage"]) != `"configure"` || string(result["vmid"]) != "201" || result["error"] == nil || len(fixture.mutations) != 2 {
		t.Fatalf("error = %v, result = %s, mutations = %v", err, result, fixture.mutations)
	}
}

func TestImageSelectionJSONNeverPrompts(t *testing.T) {
	fixture := &imageAPIFixture{}
	fixtureServer(t, fixture)
	for _, flags := range [][]string{{"--select-image"}, {"--image", "lxc:101", "--template", "local:vztmpl/ubuntu.tar.zst"}, {"--image", ""}, {"--template", " "}} {
		args := append([]string{"create", "app-02", "--json"}, flags...)
		if _, _, err := runTestCLI(t, args...); err == nil || len(fixture.mutations) != 0 {
			t.Fatalf("error = %v, mutations = %v", err, fixture.mutations)
		}
	}
}

func TestCreateFromArchiveKeepsTaskFieldsAndAddsImageMetadata(t *testing.T) {
	fixture := &imageAPIFixture{}
	fixtureServer(t, fixture)
	result, stderr, err := runTestCLI(t, "create", "app-02", "--json")
	if err != nil || stderr != "" || string(result["exitstatus"]) != `"OK"` || result["upid"] == nil || result["image"] == nil || string(result["vmid"]) != "201" {
		t.Fatalf("error = %v, result = %s, stderr = %q", err, result, stderr)
	}
	if fixture.create.Get("ostemplate") != "local:vztmpl/ubuntu.tar.zst" || fixture.create.Get("rootfs") != "local-lvm:8" || fixture.create.Get("tags") != "pxon" {
		t.Fatalf("create form = %v", fixture.create)
	}
}

func TestImagesListJSON(t *testing.T) {
	fixture := &imageAPIFixture{template: true}
	fixtureServer(t, fixture)
	result, _, err := runTestCLI(t, "images", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var data []map[string]any
	if err := json.Unmarshal(result["data"], &data); err != nil || len(data) != 2 {
		t.Fatalf("data = %s, error = %v", result["data"], err)
	}
}

func TestCreateUsesConfiguredContainerImage(t *testing.T) {
	fixture := &imageAPIFixture{template: true}
	fixtureServer(t, fixture)
	t.Setenv("PXON_DEFAULT_IMAGE", "lxc:101")
	_, _, err := runTestCLI(t, "create", "app-02", "--json")
	if err != nil || fixture.clone.Get("newid") != "201" || fixture.create != nil {
		t.Fatalf("error = %v, clone = %v, create = %v", err, fixture.clone, fixture.create)
	}
}

func TestSSHRejectsTemplateBeforeConnecting(t *testing.T) {
	fixture := &imageAPIFixture{template: true}
	fixtureServer(t, fixture)
	_, _, err := runTestCLI(t, "ssh", "configured", "--json")
	if err == nil || !strings.Contains(err.Error(), "is a template") {
		t.Fatalf("SSH template error = %v", err)
	}
}
