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

type compactPRISMTimeNode struct {
	instant  float64
	sequence uint64
	priority uint64
	size     int
	left     *compactPRISMTimeNode
	right    *compactPRISMTimeNode
}

type compactPRISMIntervalSet struct {
	starts *compactPRISMTimeNode
	ends   *compactPRISMTimeNode
}

type compactPRISMNetworkBatch struct {
	base      compactPRISMState
	intervals map[string]*compactPRISMIntervalSet
	sequence  uint64
}

type compactPRISMIntervalIndexNode struct {
	key      string
	value    *compactPRISMIntervalSet
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

func compactPRISMTimeInsert(
	root *compactPRISMTimeNode,
	instant float64,
	sequence uint64,
) *compactPRISMTimeNode {
	if root == nil {
		return &compactPRISMTimeNode{
			instant: instant, sequence: sequence,
			priority: compactPRISMPriority(int(sequence)), size: 1,
		}
	}
	copy := *root
	if instant < root.instant || (instant == root.instant && sequence < root.sequence) {
		copy.left = compactPRISMTimeInsert(root.left, instant, sequence)
		if copy.left.priority < copy.priority {
			return compactPRISMRotateTimeRight(&copy)
		}
	} else {
		copy.right = compactPRISMTimeInsert(root.right, instant, sequence)
		if copy.right.priority < copy.priority {
			return compactPRISMRotateTimeLeft(&copy)
		}
	}
	compactPRISMRefreshTime(&copy)
	return &copy
}

func compactPRISMTimeInsertMutable(
	root *compactPRISMTimeNode,
	instant float64,
	sequence uint64,
) *compactPRISMTimeNode {
	if root == nil {
		return &compactPRISMTimeNode{
			instant: instant, sequence: sequence,
			priority: compactPRISMPriority(int(sequence)), size: 1,
		}
	}
	if instant < root.instant || (instant == root.instant && sequence < root.sequence) {
		root.left = compactPRISMTimeInsertMutable(root.left, instant, sequence)
		if root.left.priority < root.priority {
			return compactPRISMRotateTimeRightMutable(root)
		}
	} else {
		root.right = compactPRISMTimeInsertMutable(root.right, instant, sequence)
		if root.right.priority < root.priority {
			return compactPRISMRotateTimeLeftMutable(root)
		}
	}
	compactPRISMRefreshTime(root)
	return root
}

func compactPRISMTimeSize(root *compactPRISMTimeNode) int {
	if root == nil {
		return 0
	}
	return root.size
}

func compactPRISMRefreshTime(root *compactPRISMTimeNode) {
	root.size = 1 + compactPRISMTimeSize(root.left) + compactPRISMTimeSize(root.right)
}

func compactPRISMRotateTimeRight(root *compactPRISMTimeNode) *compactPRISMTimeNode {
	left, updated := *root.left, *root
	updated.left = left.right
	compactPRISMRefreshTime(&updated)
	left.right = &updated
	compactPRISMRefreshTime(&left)
	return &left
}

func compactPRISMRotateTimeLeft(root *compactPRISMTimeNode) *compactPRISMTimeNode {
	right, updated := *root.right, *root
	updated.right = right.left
	compactPRISMRefreshTime(&updated)
	right.left = &updated
	compactPRISMRefreshTime(&right)
	return &right
}

func compactPRISMRotateTimeRightMutable(root *compactPRISMTimeNode) *compactPRISMTimeNode {
	left := root.left
	root.left = left.right
	compactPRISMRefreshTime(root)
	left.right = root
	compactPRISMRefreshTime(left)
	return left
}

func compactPRISMRotateTimeLeftMutable(root *compactPRISMTimeNode) *compactPRISMTimeNode {
	right := root.right
	root.right = right.left
	compactPRISMRefreshTime(root)
	right.left = root
	compactPRISMRefreshTime(right)
	return right
}

func compactPRISMTimeBefore(root *compactPRISMTimeNode, instant float64, sequence uint64) bool {
	return root.instant < instant || (root.instant == instant && root.sequence < sequence)
}

func compactPRISMTimeSplit(
	root *compactPRISMTimeNode,
	instant float64,
	sequence uint64,
) (*compactPRISMTimeNode, *compactPRISMTimeNode) {
	if root == nil {
		return nil, nil
	}
	copy := *root
	if compactPRISMTimeBefore(root, instant, sequence) {
		left, right := compactPRISMTimeSplit(root.right, instant, sequence)
		copy.right = left
		compactPRISMRefreshTime(&copy)
		return &copy, right
	}
	left, right := compactPRISMTimeSplit(root.left, instant, sequence)
	copy.left = right
	compactPRISMRefreshTime(&copy)
	return left, &copy
}

func compactPRISMTimeUnion(
	left *compactPRISMTimeNode,
	right *compactPRISMTimeNode,
) *compactPRISMTimeNode {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	if right.priority < left.priority {
		left, right = right, left
	}
	before, after := compactPRISMTimeSplit(right, left.instant, left.sequence)
	copy := *left
	copy.left = compactPRISMTimeUnion(left.left, before)
	copy.right = compactPRISMTimeUnion(left.right, after)
	compactPRISMRefreshTime(&copy)
	return &copy
}

func compactPRISMTimeCountAtOrBefore(root *compactPRISMTimeNode, instant float64) int {
	count := 0
	for root != nil {
		if root.instant > instant {
			root = root.left
			continue
		}
		count += 1 + compactPRISMTimeSize(root.left)
		root = root.right
	}
	return count
}

func compactPRISMActiveIntervals(set *compactPRISMIntervalSet, instant float64) int {
	if set == nil {
		return 0
	}
	// Intervals are [start, end): flows delivered exactly at instant are no
	// longer active, hence both counts include equality.
	return compactPRISMTimeCountAtOrBefore(set.starts, instant) -
		compactPRISMTimeCountAtOrBefore(set.ends, instant)
}

func newCompactPRISMNetworkBatch(state compactPRISMState) *compactPRISMNetworkBatch {
	return &compactPRISMNetworkBatch{
		base: state, intervals: make(map[string]*compactPRISMIntervalSet),
		sequence: state.networkSequence,
	}
}

func (batch *compactPRISMNetworkBatch) active(key string, instant float64) int {
	return compactPRISMActiveIntervals(
		compactPRISMIntervalIndexLookup(batch.base.networkIntervals, key), instant,
	) + compactPRISMActiveIntervals(batch.intervals[key], instant)
}

func (batch *compactPRISMNetworkBatch) add(
	source string,
	target string,
	readyAt float64,
	deliveredAt float64,
	route []compactPRISMRouteHop,
) {
	batch.sequence++
	keys := []string{"s:" + source, "d:" + target, "p:" + source + "\x00" + target}
	for _, hop := range route {
		keys = append(keys, hop.key)
	}
	for _, key := range keys {
		set := batch.intervals[key]
		if set == nil {
			set = &compactPRISMIntervalSet{}
			batch.intervals[key] = set
		}
		set.starts = compactPRISMTimeInsertMutable(set.starts, readyAt, batch.sequence)
		set.ends = compactPRISMTimeInsertMutable(set.ends, deliveredAt, batch.sequence)
	}
}

func (batch *compactPRISMNetworkBatch) commit() compactPRISMState {
	state := batch.base
	state.networkSequence = batch.sequence
	for key, overlay := range batch.intervals {
		base := compactPRISMIntervalIndexLookup(state.networkIntervals, key)
		merged := &compactPRISMIntervalSet{}
		if base != nil {
			merged.starts = base.starts
			merged.ends = base.ends
		}
		merged.starts = compactPRISMTimeUnion(merged.starts, overlay.starts)
		merged.ends = compactPRISMTimeUnion(merged.ends, overlay.ends)
		state.networkIntervals = compactPRISMIntervalIndexInsert(
			state.networkIntervals, key, merged,
		)
	}
	return state
}

func compactPRISMIntervalIndexLookup(
	root *compactPRISMIntervalIndexNode,
	key string,
) *compactPRISMIntervalSet {
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
	value *compactPRISMIntervalSet,
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

func compactPRISMTransferSecondsOnRouteBatch(
	batch *compactPRISMNetworkBatch,
	source string,
	target string,
	bytes int64,
	readyAt float64,
	route compactPRISMRoute,
) float64 {
	if source == target || bytes <= 0 {
		return 0
	}
	sourceActive := batch.active("s:"+source, readyAt)
	targetActive := batch.active("d:"+target, readyAt)
	duplicate := batch.active("p:"+source+"\x00"+target, readyAt)
	endpointConcurrency := 1 + sourceActive + targetActive - duplicate
	seconds := 0.0
	for _, hop := range route.hops {
		concurrency := max(endpointConcurrency, 1+batch.active(hop.key, readyAt))
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
		previous := compactPRISMIntervalIndexLookup(state.networkIntervals, key)
		intervals := &compactPRISMIntervalSet{}
		if previous != nil {
			*intervals = *previous
		}
		intervals.starts = compactPRISMTimeInsert(
			intervals.starts, readyAt, state.networkSequence,
		)
		intervals.ends = compactPRISMTimeInsert(
			intervals.ends, deliveredAt, state.networkSequence,
		)
		state.networkIntervals = compactPRISMIntervalIndexInsert(
			state.networkIntervals,
			key,
			intervals,
		)
	}
	return state
}
