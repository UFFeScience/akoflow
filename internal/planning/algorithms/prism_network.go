package algorithms

import (
	"container/heap"
	"fmt"
	"math"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type compactPRISMRouteHop struct {
	link domain.NetworkLink
	key  string
}

type compactPRISMRoute struct {
	hops []compactPRISMRouteHop
}

type compactPRISMRouteKey struct {
	source string
	target string
}

type compactPRISMCachedRoute struct {
	route  compactPRISMRoute
	exists bool
}

// compactPRISMRouter freezes every resource-to-resource route used by a
// planning request. Routes depend only on the frozen topology, not on payload
// size or search state, so rebuilding the graph during every beam expansion is
// pure duplicate work.
type compactPRISMRouter struct {
	routes map[compactPRISMRouteKey]compactPRISMCachedRoute
}

func newCompactPRISMRouter(
	topology domain.NetworkTopology,
	resources []domain.Resource,
) compactPRISMRouter {
	router := compactPRISMRouter{
		routes: make(map[compactPRISMRouteKey]compactPRISMCachedRoute, len(resources)*len(resources)),
	}
	for _, source := range resources {
		for _, target := range resources {
			if source.ID == target.ID {
				continue
			}
			route, exists := compactPRISMShortestRoute(topology, source.ID, target.ID, 0)
			router.routes[compactPRISMRouteKey{source: source.ID, target: target.ID}] =
				compactPRISMCachedRoute{route: route, exists: exists}
		}
	}
	return router
}

func (router compactPRISMRouter) route(source string, target string) (compactPRISMRoute, bool) {
	if source == target {
		return compactPRISMRoute{}, true
	}
	cached, exists := router.routes[compactPRISMRouteKey{source: source, target: target}]
	if !exists {
		return compactPRISMRoute{}, false
	}
	return cached.route, cached.exists
}

type compactPRISMRouteCandidate struct {
	node     string
	seconds  float64
	sequence int
}

type compactPRISMRouteQueue []compactPRISMRouteCandidate

func (queue compactPRISMRouteQueue) Len() int           { return len(queue) }
func (queue compactPRISMRouteQueue) Less(i, j int) bool { return queue[i].seconds < queue[j].seconds }
func (queue compactPRISMRouteQueue) Swap(i, j int)      { queue[i], queue[j] = queue[j], queue[i] }
func (queue *compactPRISMRouteQueue) Push(value any) {
	*queue = append(*queue, value.(compactPRISMRouteCandidate))
}
func (queue *compactPRISMRouteQueue) Pop() any {
	old := *queue
	value := old[len(old)-1]
	*queue = old[:len(old)-1]
	return value
}

type compactPRISMIntervalNode struct {
	readyAt     float64
	deliveredAt float64
	maximumEnd  float64
	sequence    uint64
	priority    uint64
	left        *compactPRISMIntervalNode
	right       *compactPRISMIntervalNode
}

type compactPRISMIntervalIndexNode struct {
	key      string
	value    *compactPRISMIntervalNode
	priority uint64
	left     *compactPRISMIntervalIndexNode
	right    *compactPRISMIntervalIndexNode
}

func compactPRISMStringPriority(value string) uint64 {
	const prime = uint64(1099511628211)
	hash := uint64(14695981039346656037)
	for index := 0; index < len(value); index++ {
		hash ^= uint64(value[index])
		hash *= prime
	}
	return hash
}

func compactPRISMIntervalInsert(
	root *compactPRISMIntervalNode,
	readyAt float64,
	deliveredAt float64,
	sequence uint64,
) *compactPRISMIntervalNode {
	if root == nil {
		return &compactPRISMIntervalNode{
			readyAt: readyAt, deliveredAt: deliveredAt, maximumEnd: deliveredAt,
			sequence: sequence, priority: compactPRISMPriority(int(sequence)),
		}
	}
	copy := *root
	if readyAt < root.readyAt || (readyAt == root.readyAt && sequence < root.sequence) {
		copy.left = compactPRISMIntervalInsert(root.left, readyAt, deliveredAt, sequence)
		if copy.left.priority < copy.priority {
			return compactPRISMRotateIntervalRight(&copy)
		}
	} else {
		copy.right = compactPRISMIntervalInsert(root.right, readyAt, deliveredAt, sequence)
		if copy.right.priority < copy.priority {
			return compactPRISMRotateIntervalLeft(&copy)
		}
	}
	compactPRISMRefreshInterval(&copy)
	return &copy
}

func compactPRISMRefreshInterval(root *compactPRISMIntervalNode) {
	root.maximumEnd = root.deliveredAt
	if root.left != nil {
		root.maximumEnd = math.Max(root.maximumEnd, root.left.maximumEnd)
	}
	if root.right != nil {
		root.maximumEnd = math.Max(root.maximumEnd, root.right.maximumEnd)
	}
}

func compactPRISMRotateIntervalRight(
	root *compactPRISMIntervalNode,
) *compactPRISMIntervalNode {
	left, updated := *root.left, *root
	updated.left = left.right
	compactPRISMRefreshInterval(&updated)
	left.right = &updated
	compactPRISMRefreshInterval(&left)
	return &left
}

func compactPRISMRotateIntervalLeft(
	root *compactPRISMIntervalNode,
) *compactPRISMIntervalNode {
	right, updated := *root.right, *root
	updated.right = right.left
	compactPRISMRefreshInterval(&updated)
	right.left = &updated
	compactPRISMRefreshInterval(&right)
	return &right
}

func compactPRISMActiveIntervals(root *compactPRISMIntervalNode, instant float64) int {
	if root == nil || root.maximumEnd <= instant {
		return 0
	}
	count := compactPRISMActiveIntervals(root.left, instant)
	if root.readyAt <= instant && instant < root.deliveredAt {
		count++
	}
	if root.readyAt <= instant {
		count += compactPRISMActiveIntervals(root.right, instant)
	}
	return count
}

func compactPRISMIntervalIndexLookup(
	root *compactPRISMIntervalIndexNode,
	key string,
) *compactPRISMIntervalNode {
	for root != nil {
		if key < root.key {
			root = root.left
		} else if key > root.key {
			root = root.right
		} else {
			return root.value
		}
	}
	return nil
}

func compactPRISMIntervalIndexInsert(
	root *compactPRISMIntervalIndexNode,
	key string,
	value *compactPRISMIntervalNode,
) *compactPRISMIntervalIndexNode {
	if root == nil {
		return &compactPRISMIntervalIndexNode{
			key: key, value: value, priority: compactPRISMStringPriority(key),
		}
	}
	copy := *root
	if key < root.key {
		copy.left = compactPRISMIntervalIndexInsert(root.left, key, value)
		if copy.left.priority < copy.priority {
			return compactPRISMRotateIntervalIndexRight(&copy)
		}
	} else if key > root.key {
		copy.right = compactPRISMIntervalIndexInsert(root.right, key, value)
		if copy.right.priority < copy.priority {
			return compactPRISMRotateIntervalIndexLeft(&copy)
		}
	} else {
		copy.value = value
	}
	return &copy
}

func compactPRISMRotateIntervalIndexRight(
	root *compactPRISMIntervalIndexNode,
) *compactPRISMIntervalIndexNode {
	left, updated := *root.left, *root
	updated.left = left.right
	left.right = &updated
	return &left
}

func compactPRISMRotateIntervalIndexLeft(
	root *compactPRISMIntervalIndexNode,
) *compactPRISMIntervalIndexNode {
	right, updated := *root.right, *root
	updated.right = right.left
	right.left = &updated
	return &right
}

func compactPRISMTransferSeconds(
	topology domain.NetworkTopology,
	state compactPRISMState,
	source string,
	target string,
	bytes int64,
	readyAt float64,
) float64 {
	if source == target || bytes <= 0 {
		return 0
	}
	route, exists := compactPRISMShortestRoute(topology, source, target, bytes)
	if !exists {
		return math.Inf(1)
	}
	return compactPRISMTransferSecondsOnRoute(state, source, target, bytes, readyAt, route)
}

func compactPRISMTransferSecondsOnRoute(
	state compactPRISMState,
	source string,
	target string,
	bytes int64,
	readyAt float64,
	route compactPRISMRoute,
) float64 {
	if source == target || bytes <= 0 {
		return 0
	}
	sourceActive := compactPRISMActiveIntervals(
		compactPRISMIntervalIndexLookup(state.networkIntervals, "s:"+source), readyAt,
	)
	targetActive := compactPRISMActiveIntervals(
		compactPRISMIntervalIndexLookup(state.networkIntervals, "d:"+target), readyAt,
	)
	duplicate := compactPRISMActiveIntervals(
		compactPRISMIntervalIndexLookup(state.networkIntervals, "p:"+source+"\x00"+target), readyAt,
	)
	endpointConcurrency := 1 + sourceActive + targetActive - duplicate
	seconds := 0.0
	for _, hop := range route.hops {
		linkConcurrency := 1 + compactPRISMActiveIntervals(
			compactPRISMIntervalIndexLookup(state.networkIntervals, hop.key), readyAt,
		)
		concurrency := max(endpointConcurrency, linkConcurrency)
		if hop.link.BandwidthBitsPerSecond <= 0 {
			return math.Inf(1)
		}
		dataSeconds := float64(bytes) / (hop.link.BandwidthBitsPerSecond / 8.0)
		seconds += hop.link.LatencySeconds + dataSeconds*float64(concurrency)
	}
	return seconds
}

func compactPRISMShortestRoute(
	topology domain.NetworkTopology,
	source string,
	target string,
	bytes int64,
) (compactPRISMRoute, bool) {
	type edge struct {
		to  string
		hop compactPRISMRouteHop
	}
	graph := make(map[string][]edge)
	for index, link := range topology.Links {
		key := link.ID
		if key == "" {
			key = fmt.Sprintf("%d:%s:%s", index, link.SourceResourceID, link.TargetResourceID)
		}
		hop := compactPRISMRouteHop{link: link, key: "l:" + key}
		graph[link.SourceResourceID] = append(graph[link.SourceResourceID], edge{to: link.TargetResourceID, hop: hop})
		if link.Bidirectional {
			graph[link.TargetResourceID] = append(graph[link.TargetResourceID], edge{to: link.SourceResourceID, hop: hop})
		}
	}
	distance := map[string]float64{source: 0}
	previousNode := make(map[string]string)
	previousHop := make(map[string]compactPRISMRouteHop)
	queue := &compactPRISMRouteQueue{{node: source}}
	heap.Init(queue)
	sequence := 0
	for queue.Len() > 0 {
		candidate := heap.Pop(queue).(compactPRISMRouteCandidate)
		if candidate.seconds != distance[candidate.node] {
			continue
		}
		if candidate.node == target {
			break
		}
		for _, edge := range graph[candidate.node] {
			// SimGrid's Full-routing platform selects a route using link latency
			// plus the transmission time of one byte, independently of payload.
			weight := edge.hop.link.LatencySeconds +
				8/math.Max(edge.hop.link.BandwidthBitsPerSecond, 1)
			if edge.hop.link.BandwidthBitsPerSecond <= 0 {
				continue
			}
			next := candidate.seconds + weight
			old, exists := distance[edge.to]
			if exists && old <= next {
				continue
			}
			distance[edge.to] = next
			previousNode[edge.to] = candidate.node
			previousHop[edge.to] = edge.hop
			sequence++
			heap.Push(queue, compactPRISMRouteCandidate{node: edge.to, seconds: next, sequence: sequence})
		}
	}
	if _, exists := distance[target]; !exists {
		return compactPRISMRoute{}, false
	}
	hops := make([]compactPRISMRouteHop, 0)
	for node := target; node != source; node = previousNode[node] {
		hops = append(hops, previousHop[node])
	}
	for left, right := 0, len(hops)-1; left < right; left, right = left+1, right-1 {
		hops[left], hops[right] = hops[right], hops[left]
	}
	return compactPRISMRoute{hops: hops}, true
}

func compactPRISMAddNetworkFlow(
	state compactPRISMState,
	source string,
	target string,
	readyAt float64,
	deliveredAt float64,
) compactPRISMState {
	return compactPRISMAddNetworkFlowOnRoute(state, source, target, readyAt, deliveredAt, nil)
}

func compactPRISMAddNetworkFlowOnRoute(
	state compactPRISMState,
	source string,
	target string,
	readyAt float64,
	deliveredAt float64,
	route []compactPRISMRouteHop,
) compactPRISMState {
	state.networkSequence++
	keys := []string{
		"s:" + source,
		"d:" + target,
		"p:" + source + "\x00" + target,
	}
	for _, hop := range route {
		keys = append(keys, hop.key)
	}
	for _, key := range keys {
		intervals := compactPRISMIntervalIndexLookup(state.networkIntervals, key)
		intervals = compactPRISMIntervalInsert(
			intervals,
			readyAt,
			deliveredAt,
			state.networkSequence,
		)
		state.networkIntervals = compactPRISMIntervalIndexInsert(
			state.networkIntervals,
			key,
			intervals,
		)
	}
	return state
}
