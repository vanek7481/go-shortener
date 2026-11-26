package storage

import (
	"sync"
	"testing"
)

func TestConcurrentStorage(t *testing.T) {
	m := New()
	const count = 200
	ids := make(chan string, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := m.Create("user", "https://example.com")
			if original, ok := m.Get(id); !ok || original != "https://example.com" {
				t.Error("missing link")
			}
			_ = m.List("user")
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)
	seen := make(map[string]bool)
	for id := range ids {
		if seen[id] {
			t.Fatal("ID collision")
		}
		seen[id] = true
	}
	links := m.List("user")
	if len(links) != count || len(m.List("other")) != 0 {
		t.Fatal("incorrect user index")
	}
	links[0].OriginalURL = "changed"
	if m.List("user")[0].OriginalURL == "changed" {
		t.Fatal("List exposes internal storage")
	}
}
