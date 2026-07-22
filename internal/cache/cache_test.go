package cache

import "testing"

func TestNew_MemoryDriver(t *testing.T) {
	d, err := New(Config{Driver: "memory", Memory: MemoryConfig{MaxEntries: 50}})
	if err != nil {
		t.Fatalf("New memory: %v", err)
	}
	if _, ok := d.(*MemoryDriver); !ok {
		t.Fatalf("expected *MemoryDriver, got %T", d)
	}
}

func TestNew_DefaultsToMemory(t *testing.T) {
	d, err := New(Config{Driver: ""})
	if err != nil {
		t.Fatalf("New default: %v", err)
	}
	if _, ok := d.(*MemoryDriver); !ok {
		t.Fatalf("expected *MemoryDriver for empty driver, got %T", d)
	}
}

func TestNew_UnknownDriver(t *testing.T) {
	if _, err := New(Config{Driver: "memcached"}); err == nil {
		t.Fatal("expected error for unknown driver")
	}
}

// TestNew_RedisUnreachable verifies the factory surfaces a connection error
// (instead of returning a broken driver) when Redis cannot be reached.
func TestNew_RedisUnreachable(t *testing.T) {
	_, err := New(Config{
		Driver: "redis",
		Redis:  RedisConfig{Addr: "127.0.0.1:1", Prefix: "test:"},
	})
	if err == nil {
		t.Fatal("expected error when redis is unreachable")
	}
}
