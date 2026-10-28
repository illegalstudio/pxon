package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWaitForTaskDeadlineInterruptsPollingSleepAndPreservesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"upid":"UPID:power","status":"running"}}`)
	}))
	defer server.Close()
	client := &Client{endpoint: server.URL, httpClient: server.Client()}
	started := time.Now()
	status, err := client.WaitForTask("pve", "UPID:power", 30*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) || status == nil || status.Status != "running" || time.Since(started) > time.Second {
		t.Fatalf("status=%v, err=%v, elapsed=%s", status, err, time.Since(started))
	}
}

func TestLifecycleClientValidatesBeforeSendingRequests(t *testing.T) {
	client := &Client{}
	for _, target := range []struct {
		node string
		vmid int
	}{{"", 101}, {"pve", 0}, {"pve", 1000000000}} {
		if _, err := client.StartContainer(target.node, target.vmid); err == nil {
			t.Fatal("start accepted invalid target")
		}
		if _, err := client.ShutdownContainer(target.node, target.vmid, time.Minute); err == nil {
			t.Fatal("shutdown accepted invalid target")
		}
		if _, err := client.ContainerState(target.node, target.vmid); err == nil {
			t.Fatal("status accepted invalid target")
		}
	}
	if _, err := client.ShutdownContainer("pve", 101, 0); err == nil {
		t.Fatal("shutdown accepted non-positive timeout")
	}
}

func TestLifecycleClientRejectsMissingStatusAndEmptyTaskIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"data":{}}`)
		} else {
			fmt.Fprint(w, `{"data":""}`)
		}
	}))
	defer server.Close()
	client := &Client{endpoint: server.URL, httpClient: server.Client()}
	if _, err := client.ContainerState("pve", 101); err == nil {
		t.Fatal("missing status accepted")
	}
	if _, err := client.StartContainer("pve", 101); err == nil {
		t.Fatal("empty start task accepted")
	}
	if _, err := client.ShutdownContainer("pve", 101, time.Minute); err == nil {
		t.Fatal("empty shutdown task accepted")
	}
}
