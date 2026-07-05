package cluster

import (
	"fmt"
	"math"
	"sync"
	"testing"
)

func TestHashRingBasicBehavior(t *testing.T) {
	ring := NewHashRing(100)
	for _, node := range []string{"node-a", "node-b", "node-c"} {
		ring.AddNode(node)
	}

	first, ok := ring.GetNode("https://example.com/articles/consistent-hashing")
	if !ok {
		t.Fatal("GetNode returned ok=false for a non-empty ring")
	}
	for i := 0; i < 20; i++ {
		got, ok := ring.GetNode("https://example.com/articles/consistent-hashing")
		if !ok {
			t.Fatal("GetNode returned ok=false for a non-empty ring")
		}
		if got != first {
			t.Fatalf("GetNode returned %q after %q for the same key", got, first)
		}
	}

	keys := makeKeys(10_000)
	beforeAdd := assignments(t, ring, keys)
	ring.AddNode("node-d")
	afterAdd := assignments(t, ring, keys)

	movedAfterAdd := movedCount(beforeAdd, afterAdd)
	if movedAfterAdd >= len(keys)/2 {
		t.Fatalf("adding one node moved %d/%d keys; want most keys to stay", movedAfterAdd, len(keys))
	}

	beforeRemove := afterAdd
	ring.RemoveNode("node-d")
	afterRemove := assignments(t, ring, keys)

	movedUnrelated := 0
	for _, key := range keys {
		if beforeRemove[key] != "node-d" && beforeRemove[key] != afterRemove[key] {
			movedUnrelated++
		}
	}
	if movedUnrelated != 0 {
		t.Fatalf("removing node-d moved %d keys that were not assigned to node-d", movedUnrelated)
	}
}

func TestHashRingRelocationRateWhenAddingNode(t *testing.T) {
	const (
		keyCount = 100_000
		replicas = 300
	)

	ring := NewHashRing(replicas)
	for i := 0; i < 10; i++ {
		ring.AddNode(fmt.Sprintf("node-%02d", i))
	}

	keys := makeKeys(keyCount)
	before := assignments(t, ring, keys)

	ring.AddNode("node-10")
	after := assignments(t, ring, keys)

	moved := movedCount(before, after)
	rate := float64(moved) / float64(keyCount)
	want := 1.0 / 11.0
	const tolerance = 0.03
	if math.Abs(rate-want) > tolerance {
		t.Fatalf("move rate = %.4f (%d/%d), want near %.4f +/- %.2f", rate, moved, keyCount, want, tolerance)
	}
}

func TestHashRingReplicasImproveDistribution(t *testing.T) {
	const keyCount = 100_000
	keys := makeKeys(keyCount)

	oneReplica := NewHashRing(1)
	manyReplicas := NewHashRing(200)
	nodes := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		node := fmt.Sprintf("node-%02d", i)
		nodes = append(nodes, node)
		oneReplica.AddNode(node)
		manyReplicas.AddNode(node)
	}

	oneReplicaStddev := assignmentStddev(t, oneReplica, keys, nodes)
	manyReplicasStddev := assignmentStddev(t, manyReplicas, keys, nodes)

	if manyReplicasStddev >= oneReplicaStddev*0.60 {
		t.Fatalf("stddev with many replicas = %.2f, with one replica = %.2f; want many replicas to reduce skew clearly", manyReplicasStddev, oneReplicaStddev)
	}
}

func TestHashRingConcurrentAddAndGet(t *testing.T) {
	ring := NewHashRing(50)
	ring.AddNode("seed")

	const (
		readerCount = 8
		iterations  = 5_000
		nodesToAdd  = 200
	)

	start := make(chan struct{})
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr string
	report := func(format string, args ...any) {
		once.Do(func() {
			firstErr = fmt.Sprintf(format, args...)
		})
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < nodesToAdd; i++ {
			ring.AddNode(fmt.Sprintf("node-%03d", i))
		}
	}()

	for reader := 0; reader < readerCount; reader++ {
		wg.Add(1)
		go func(reader int) {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("reader-%02d-key-%05d", reader, i)
				if _, ok := ring.GetNode(key); !ok {
					report("GetNode(%q) returned ok=false while ring has nodes", key)
					return
				}
			}
		}(reader)
	}

	close(start)
	wg.Wait()

	if firstErr != "" {
		t.Fatal(firstErr)
	}
}

func makeKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%06d", i)
	}
	return keys
}

func assignments(t *testing.T, ring *HashRing, keys []string) map[string]string {
	t.Helper()

	result := make(map[string]string, len(keys))
	for _, key := range keys {
		node, ok := ring.GetNode(key)
		if !ok {
			t.Fatalf("GetNode(%q) returned ok=false", key)
		}
		result[key] = node
	}
	return result
}

func movedCount(before, after map[string]string) int {
	moved := 0
	for key, beforeNode := range before {
		if after[key] != beforeNode {
			moved++
		}
	}
	return moved
}

func assignmentStddev(t *testing.T, ring *HashRing, keys []string, nodes []string) float64 {
	t.Helper()

	expected := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		if _, ok := expected[node]; ok {
			t.Fatalf("duplicate expected node %q", node)
		}
		expected[node] = struct{}{}
	}

	counts := make(map[string]int)
	for _, key := range keys {
		node, ok := ring.GetNode(key)
		if !ok {
			t.Fatalf("GetNode(%q) returned ok=false", key)
		}
		if _, ok := expected[node]; !ok {
			t.Fatalf("GetNode(%q) returned unexpected node %q", key, node)
		}
		counts[node]++
	}

	if len(counts) != len(nodes) {
		t.Fatalf("assigned nodes = %d, want %d; counts=%v", len(counts), len(nodes), counts)
	}

	mean := float64(len(keys)) / float64(len(nodes))
	var sumSquared float64
	for _, node := range nodes {
		count := counts[node]
		diff := float64(count) - mean
		sumSquared += diff * diff
	}
	return math.Sqrt(sumSquared / float64(len(nodes)))
}
