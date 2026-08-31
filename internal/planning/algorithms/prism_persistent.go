package algorithms

import (
	"math"
)

type compactPRISMAssignment struct {
	valid            bool
	activityOrdinal  int
	resourceOrdinal  int
	coreOrdinal      int
	sequence         int
	order            int
	priority         int
	readyAt          float64
	startAt          float64
	finishAt         float64
	runtimeSeconds   float64
	transferSeconds  float64
	cost             float64
	bootSeconds      float64
	containerSeconds float64
	queueSeconds     float64
	transferCost     float64
}

type compactPRISMAssignmentTrace struct {
	assignment compactPRISMAssignment
	previous   *compactPRISMAssignmentTrace
	length     int
}

func compactPRISMAssignmentChunkLookup(
	chunks [][]compactPRISMAssignment,
	key int,
) (compactPRISMAssignment, bool) {
	chunkIndex := key / prismPendingChunkSize
	offset := key % prismPendingChunkSize
	if chunkIndex < 0 || chunkIndex >= len(chunks) || chunks[chunkIndex] == nil {
		return compactPRISMAssignment{}, false
	}
	assignment := chunks[chunkIndex][offset]
	if !assignment.valid {
		return compactPRISMAssignment{}, false
	}
	return assignment, true
}

func compactPRISMAssignmentChunkInsert(
	chunks [][]compactPRISMAssignment,
	key int,
	value compactPRISMAssignment,
) [][]compactPRISMAssignment {
	chunkIndex := key / prismPendingChunkSize
	offset := key % prismPendingChunkSize
	updated := append([][]compactPRISMAssignment(nil), chunks...)
	chunk := make([]compactPRISMAssignment, prismPendingChunkSize)
	if updated[chunkIndex] != nil {
		copy(chunk, updated[chunkIndex])
	}
	value.valid = true
	chunk[offset] = value
	updated[chunkIndex] = chunk
	return updated
}

type compactPRISMIntNode struct {
	key      int
	value    int
	priority uint64
	left     *compactPRISMIntNode
	right    *compactPRISMIntNode
}

type compactPRISMCoreNode struct {
	key       int
	value     float64
	priority  uint64
	bestKey   int
	bestValue float64
	left      *compactPRISMCoreNode
	right     *compactPRISMCoreNode
}

func compactPRISMPriority(key int) uint64 {
	value := uint64(key + 1)
	value ^= value >> 30
	value *= 0xbf58476d1ce4e5b9
	value ^= value >> 27
	value *= 0x94d049bb133111eb
	return value ^ (value >> 31)
}

func compactPRISMIntLookup(root *compactPRISMIntNode, key int) int {
	for root != nil {
		if key < root.key {
			root = root.left
		} else if key > root.key {
			root = root.right
		} else {
			return root.value
		}
	}
	return 0
}

func compactPRISMIntInsert(
	root *compactPRISMIntNode,
	key int,
	value int,
) *compactPRISMIntNode {
	if root == nil {
		return &compactPRISMIntNode{
			key: key, value: value, priority: compactPRISMPriority(key),
		}
	}
	copy := *root
	if key < root.key {
		copy.left = compactPRISMIntInsert(root.left, key, value)
		if copy.left.priority < copy.priority {
			return compactPRISMRotateIntRight(&copy)
		}
	} else if key > root.key {
		copy.right = compactPRISMIntInsert(root.right, key, value)
		if copy.right.priority < copy.priority {
			return compactPRISMRotateIntLeft(&copy)
		}
	} else {
		copy.value = value
	}
	return &copy
}

func compactPRISMRotateIntRight(root *compactPRISMIntNode) *compactPRISMIntNode {
	left, updated := *root.left, *root
	updated.left = left.right
	left.right = &updated
	return &left
}

func compactPRISMRotateIntLeft(root *compactPRISMIntNode) *compactPRISMIntNode {
	right, updated := *root.right, *root
	updated.right = right.left
	right.left = &updated
	return &right
}

func compactPRISMCoreInsert(
	root *compactPRISMCoreNode,
	key int,
	value float64,
) *compactPRISMCoreNode {
	if root == nil {
		return &compactPRISMCoreNode{
			key: key, value: value, priority: compactPRISMPriority(key),
			bestKey: key, bestValue: value,
		}
	}
	copy := *root
	if key < root.key {
		copy.left = compactPRISMCoreInsert(root.left, key, value)
		if copy.left.priority < copy.priority {
			return compactPRISMRotateCoreRight(&copy)
		}
	} else if key > root.key {
		copy.right = compactPRISMCoreInsert(root.right, key, value)
		if copy.right.priority < copy.priority {
			return compactPRISMRotateCoreLeft(&copy)
		}
	} else {
		copy.value = value
	}
	compactPRISMRefreshCore(&copy)
	return &copy
}

func compactPRISMRefreshCore(root *compactPRISMCoreNode) {
	root.bestKey, root.bestValue = root.key, root.value
	for _, child := range []*compactPRISMCoreNode{root.left, root.right} {
		if child == nil {
			continue
		}
		if child.bestValue < root.bestValue ||
			(child.bestValue == root.bestValue && child.bestKey < root.bestKey) {
			root.bestKey, root.bestValue = child.bestKey, child.bestValue
		}
	}
}

func compactPRISMRotateCoreRight(root *compactPRISMCoreNode) *compactPRISMCoreNode {
	left, updated := *root.left, *root
	updated.left = left.right
	compactPRISMRefreshCore(&updated)
	left.right = &updated
	compactPRISMRefreshCore(&left)
	return &left
}

func compactPRISMRotateCoreLeft(root *compactPRISMCoreNode) *compactPRISMCoreNode {
	right, updated := *root.right, *root
	updated.right = right.left
	compactPRISMRefreshCore(&updated)
	right.left = &updated
	compactPRISMRefreshCore(&right)
	return &right
}

func compactPRISMInitialCoreRoots(search compactPRISMContext) []*compactPRISMCoreNode {
	roots := make([]*compactPRISMCoreNode, len(search.resources))
	for resourceOrdinal, resource := range search.resources {
		for coreOrdinal := range resource.cores {
			roots[resourceOrdinal] = compactPRISMCoreInsert(
				roots[resourceOrdinal],
				resource.coreOffset+coreOrdinal,
				0,
			)
		}
	}
	return roots
}

func compactPRISMCoreAvailable(root *compactPRISMCoreNode, key int) float64 {
	for root != nil {
		if key < root.key {
			root = root.left
		} else if key > root.key {
			root = root.right
		} else {
			return root.value
		}
	}
	return math.Inf(1)
}
