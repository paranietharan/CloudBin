package service

import (
	"fmt"
	"hash/fnv"
	"slices"
	"strings"
)

type HashRing struct {
	nodes        []string
	virtualNodes map[uint64]string
	keys         []uint64
}

func NewHashRing(nodes []string, replicasPerNode int) *HashRing {
	hr := &HashRing{
		nodes:        nodes,
		virtualNodes: make(map[uint64]string),
	}
	if replicasPerNode <= 0 {
		replicasPerNode = 50
	}
	for _, node := range nodes {
		for i := 0; i < replicasPerNode; i++ {
			vKey := fmt.Sprintf("%s#%d", node, i)
			h := fnv.New64a()
			_, _ = h.Write([]byte(vKey))
			sum := h.Sum64()
			hr.virtualNodes[sum] = node
			hr.keys = append(hr.keys, sum)
		}
	}
	slices.Sort(hr.keys)
	return hr
}

func (hr *HashRing) GetNodes(key string, count int) []string {
	if len(hr.nodes) == 0 || count <= 0 {
		return nil
	}
	if count > len(hr.nodes) {
		count = len(hr.nodes)
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	target := h.Sum64()

	idx, _ := slices.BinarySearch(hr.keys, target)

	selected := make([]string, 0, count)
	seen := make(map[string]bool)

	for i := 0; i < len(hr.keys) && len(selected) < count; i++ {
		currIdx := (idx + i) % len(hr.keys)
		node := hr.virtualNodes[hr.keys[currIdx]]
		if !seen[node] {
			seen[node] = true
			selected = append(selected, node)
		}
	}
	return selected
}

func storagePath(ownerID, objectKey string) string {
	owner := strings.TrimSpace(ownerID)
	key := strings.TrimPrefix(strings.TrimSpace(objectKey), "/")
	return owner + "/" + key
}
