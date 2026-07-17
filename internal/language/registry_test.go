package language

import "testing"

func TestLoadCatalogIncludesNodeDefaultLimits(t *testing.T) {
	registry, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	node, ok := registry.Get("node-22")
	if !ok {
		t.Fatal("node-22 language not found")
	}
	if node.DefaultLimits == nil {
		t.Fatal("node-22 default limits are nil")
	}
	if node.DefaultLimits.MemoryKB != 2097152 {
		t.Fatalf("MemoryKB = %d, want 2097152", node.DefaultLimits.MemoryKB)
	}
	if node.DefaultLimits.MaxProcesses != 256 {
		t.Fatalf("MaxProcesses = %d, want 256", node.DefaultLimits.MaxProcesses)
	}
}
