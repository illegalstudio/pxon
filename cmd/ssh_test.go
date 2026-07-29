package cmd

import (
	"testing"

	"pxon/internal/proxmox"
)

func TestFindContainer(t *testing.T) {
	containers := []proxmox.Container{
		{Name: "web", VMID: 101},
		{Name: "worker", VMID: 102},
	}

	byName, err := findContainer(containers, "WEB")
	if err != nil {
		t.Fatal(err)
	}
	if byName.VMID != 101 {
		t.Fatalf("VMID = %d, want 101", byName.VMID)
	}

	byVMID, err := findContainer(containers, "102")
	if err != nil {
		t.Fatal(err)
	}
	if byVMID.Name != "worker" {
		t.Fatalf("Name = %q, want worker", byVMID.Name)
	}
}

func TestFindContainerRejectsAmbiguousName(t *testing.T) {
	containers := []proxmox.Container{
		{Name: "web", VMID: 101},
		{Name: "WEB", VMID: 102},
	}

	if _, err := findContainer(containers, "web"); err == nil {
		t.Fatal("expected ambiguous name error")
	}
}
