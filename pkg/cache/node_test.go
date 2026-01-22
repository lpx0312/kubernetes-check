package cache

import (
	"sync"
	"testing"
)

func TestNodeCache_GetNodeIP(t *testing.T) {
	cache := &NodeCache{}

	// Test empty cache
	ip := cache.GetNodeIP("node-1")
	if ip != "" {
		t.Errorf("Empty cache should return empty string, got %q", ip)
	}

	// Test after storing
	cache.Store("node-1", "192.168.1.10")
	ip = cache.GetNodeIP("node-1")
	if ip != "192.168.1.10" {
		t.Errorf("Expected 192.168.1.10, got %q", ip)
	}

	// Test non-existent node
	ip = cache.GetNodeIP("node-2")
	if ip != "" {
		t.Errorf("Non-existent node should return empty string, got %q", ip)
	}

	// Test empty node name
	ip = cache.GetNodeIP("")
	if ip != "" {
		t.Errorf("Empty node name should return empty string, got %q", ip)
	}
}

func TestNodeCache_GetNodeIPOrUnknown(t *testing.T) {
	cache := &NodeCache{}

	// Test empty cache returns "Unknown"
	ip := cache.GetNodeIPOrUnknown("node-1")
	if ip != "Unknown" {
		t.Errorf("Empty cache should return 'Unknown', got %q", ip)
	}

	// Test after storing
	cache.Store("node-1", "192.168.1.10")
	ip = cache.GetNodeIPOrUnknown("node-1")
	if ip != "192.168.1.10" {
		t.Errorf("Expected 192.168.1.10, got %q", ip)
	}

	// Test non-existent node returns "Unknown"
	ip = cache.GetNodeIPOrUnknown("node-2")
	if ip != "Unknown" {
		t.Errorf("Non-existent node should return 'Unknown', got %q", ip)
	}
}

func TestNodeCache_Concurrent(t *testing.T) {
	cache := &NodeCache{}
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			nodeName := "node-" + string(rune('0'+n%10))
			ip := "192.168.1." + string(rune('0'+n%10))
			cache.Store(nodeName, ip)
			cache.GetNodeIP(nodeName)
			cache.GetNodeIPOrUnknown(nodeName)
		}(i)
	}

	wg.Wait()
}
