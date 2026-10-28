package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type lifecycleFixture struct {
	mu        sync.Mutex
	state     string
	tags      string
	template  int
	failTask  bool
	keepState bool
	blockAt   string
	mutations []string
	shutdown  url.Values
}

func (f *lifecycleFixture) serve(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if r.URL.Path == f.blockAt {
		<-r.Context().Done()
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/cluster/resources":
		// Deliberately stale: lifecycle must use status/current instead.
		fmt.Fprintf(w, `{"data":[{"vmid":101,"name":"app","node":"pve","type":"lxc","status":"unknown","template":%d,"tags":%q}]}`, f.template, f.tags)
	case "/nodes/pve/lxc/101/config":
		fmt.Fprint(w, `{"data":{"net0":"name=eth0,ip=dhcp"}}`)
	case "/nodes/pve/lxc/101/status/current":
		fmt.Fprintf(w, `{"data":{"status":%q,"uptime":42}}`, f.state)
	case "/nodes/pve/lxc/101/status/start", "/nodes/pve/lxc/101/status/shutdown":
		if r.Method != http.MethodPost {
			t.Errorf("unexpected mutation method %s", r.Method)
		}
		f.mutations = append(f.mutations, r.URL.Path)
		_ = r.ParseForm()
		f.shutdown = r.PostForm
		if !f.failTask && !f.keepState {
			f.state = "running"
			if strings.HasSuffix(r.URL.Path, "/shutdown") {
				f.state = "stopped"
			}
		}
		fmt.Fprint(w, `{"data":"UPID:power"}`)
	case "/nodes/pve/tasks/UPID:power/status":
		exit := "OK"
		if f.failTask {
			exit = "shutdown failed"
		}
		fmt.Fprintf(w, `{"data":{"upid":"UPID:power","status":"stopped","exitstatus":%q}}`, exit)
	default:
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
		http.NotFound(w, r)
	}
}

func lifecycleServer(t *testing.T, f *lifecycleFixture) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.serve(t, w, r) }))
	t.Cleanup(server.Close)
	cliTestEnvironment(t, server.URL)
}

func TestLifecycleCurrentStatusDoesNotNeedSSHOrStaticIP(t *testing.T) {
	f := &lifecycleFixture{state: "running", tags: "app;pxon"}
	lifecycleServer(t, f)
	result, stderr, err := runTestCLI(t, "status", "app", "--json")
	if err != nil || stderr != "" || string(result["status"]) != `"running"` || string(result["changed"]) != "false" || result["ip"] != nil {
		t.Fatalf("result=%s, stderr=%q, err=%v", result, stderr, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.mutations) != 0 {
		t.Fatalf("status mutated container: %v", f.mutations)
	}
}

func TestLifecycleStartAndStopAreRepeatable(t *testing.T) {
	for _, operation := range []string{"start", "stop"} {
		t.Run(operation, func(t *testing.T) {
			initial, wanted := "stopped", "running"
			if operation == "stop" {
				initial, wanted = wanted, initial
			}
			f := &lifecycleFixture{state: initial, tags: "pxon"}
			lifecycleServer(t, f)
			result, stderr, err := runTestCLI(t, operation, "101", "--json")
			if err != nil || stderr != "" || string(result["status"]) != fmt.Sprintf("%q", wanted) || string(result["changed"]) != "true" || result["task"] == nil {
				t.Fatalf("result=%s, stderr=%q, err=%v", result, stderr, err)
			}
			result, _, err = runTestCLI(t, operation, "app", "--json")
			if err != nil || string(result["changed"]) != "false" || result["upid"] != nil {
				t.Fatalf("repeat result=%s, err=%v", result, err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.mutations) != 1 {
				t.Fatalf("mutations=%v", f.mutations)
			}
			if operation == "stop" && (!strings.HasSuffix(f.mutations[0], "/shutdown") || f.shutdown.Get("forceStop") != "0" || f.shutdown.Get("timeout") != "60") {
				t.Fatalf("unsafe shutdown: %v, %v", f.mutations, f.shutdown)
			}
		})
	}
}

func TestLifecycleRejectsUnmanagedGuestsAndTemplatesBeforeMutation(t *testing.T) {
	for _, operation := range []string{"status", "start", "stop"} {
		for _, boundary := range []string{"unmanaged", "template"} {
			t.Run(operation+"/"+boundary, func(t *testing.T) {
				f := &lifecycleFixture{state: "stopped", tags: "pxon-other"}
				if boundary == "template" {
					f.tags, f.template = "pxon", 1
				}
				lifecycleServer(t, f)
				result, _, err := runTestCLI(t, operation, "101", "--json")
				if err == nil || result["error"] == nil {
					t.Fatalf("result=%s, err=%v", result, err)
				}
				f.mu.Lock()
				defer f.mu.Unlock()
				if len(f.mutations) != 0 {
					t.Fatalf("mutations=%v", f.mutations)
				}
			})
		}
	}
}

func TestLifecycleFailedTaskPreservesIdentityAndTask(t *testing.T) {
	f := &lifecycleFixture{state: "running", tags: "pxon", failTask: true}
	lifecycleServer(t, f)
	result, _, err := runTestCLI(t, "stop", "101", "--json")
	if err == nil || result["error"] == nil || result["task"] == nil || string(result["vmid"]) != "101" || string(result["upid"]) != `"UPID:power"` || string(result["status"]) != `"running"` {
		t.Fatalf("result=%s, err=%v", result, err)
	}
}

func TestLifecycleChecksFinalStateAfterSuccessfulTask(t *testing.T) {
	f := &lifecycleFixture{state: "stopped", tags: "pxon", keepState: true}
	lifecycleServer(t, f)
	result, _, err := runTestCLI(t, "start", "app", "--json")
	if err == nil || !strings.Contains(err.Error(), "expected") || result["task"] == nil {
		t.Fatalf("result=%s, err=%v", result, err)
	}
}

func TestLifecycleOverallDeadlineCancelsLookupAndPollingRequests(t *testing.T) {
	for _, path := range []string{"/cluster/resources", "/nodes/pve/tasks/UPID:power/status"} {
		t.Run(path, func(t *testing.T) {
			f := &lifecycleFixture{state: "stopped", tags: "pxon", blockAt: path}
			lifecycleServer(t, f)
			started := time.Now()
			result, _, err := runTestCLI(t, "start", "app", "--timeout", "50ms", "--json")
			if err == nil || result["error"] == nil || time.Since(started) > time.Second {
				t.Fatalf("result=%s, err=%v, elapsed=%s", result, err, time.Since(started))
			}
			if strings.Contains(path, "tasks") && string(result["upid"]) != `"UPID:power"` {
				t.Fatalf("lost accepted task: %s", result)
			}
		})
	}
}

func TestLifecycleShutdownTimeoutAndArgumentValidation(t *testing.T) {
	f := &lifecycleFixture{state: "running", tags: "pxon"}
	lifecycleServer(t, f)
	for _, args := range [][]string{
		{"start", "--json"}, {"status", "app", "extra", "--json"},
		{"start", "app", "--timeout", "0s", "--json"},
		{"stop", "app", "--shutdown-timeout", "-1s", "--json"},
	} {
		if _, _, err := runTestCLI(t, args...); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
	result, _, err := runTestCLI(t, "stop", "app", "--shutdown-timeout", "1500ms", "--json")
	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil || f.shutdown.Get("timeout") != "2" || len(f.mutations) != 1 {
		t.Fatalf("result=%s, err=%v, shutdown=%v", result, err, f.shutdown)
	}
}
