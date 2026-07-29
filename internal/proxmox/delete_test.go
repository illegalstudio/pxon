package proxmox

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartDeleteContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.EscapedPath() != "/nodes/node%201/lxc/123" {
			t.Errorf("path = %s, want /nodes/node%%201/lxc/123", r.URL.EscapedPath())
		}
		if r.URL.Query().Get("force") != "1" {
			t.Errorf("force = %q, want 1", r.URL.Query().Get("force"))
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":"UPID:node1:delete"}`)
	}))
	defer server.Close()

	client := &Client{
		endpoint:   server.URL,
		httpClient: server.Client(),
	}

	upid, err := client.StartDeleteContainer("node 1", 123, true)
	if err != nil {
		t.Fatal(err)
	}
	if upid != "UPID:node1:delete" {
		t.Fatalf("UPID = %q", upid)
	}
}

func TestStartDeleteContainerValidatesTarget(t *testing.T) {
	client := &Client{}

	if _, err := client.StartDeleteContainer("", 123, false); err == nil {
		t.Fatal("expected missing node error")
	}
	if _, err := client.StartDeleteContainer("node1", 0, false); err == nil {
		t.Fatal("expected invalid VMID error")
	}
}
