package algorithms

import (
	"fmt"
	"sort"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const prismPendingChunkSize = 64

type compactPRISMDependency struct {
	predecessor int
	bytes       int64
}

type compactPRISMResource struct {
	resource   domain.Resource
	cores      []string
	coreOffset int
}

type compactPRISMContext struct {
	request          domain.PlanningRequest
	activities       []domain.Activity
	activityOrdinal  map[string]int
	predecessors     [][]compactPRISMDependency
	successors       [][]int
	resources        []compactPRISMResource
	router           compactPRISMRouter
	durations        [][]float64
	feasible         [][]bool
	minimumCosts     []float64
	averageDurations []float64
	ranks            []float64
	readyBranchLimit int
	beamWidth        int
	coreCount        int
}

func newCompactPRISMContext(
	request domain.PlanningRequest,
	configuration map[string]any,
) (compactPRISMContext, error) {
	topological, err := topologicalOrder(request.Workflow)
	if err != nil {
		return compactPRISMContext{}, err
	}
	resources := schedulableResources(request)
	if len(resources) == 0 {
		return compactPRISMContext{}, fmt.Errorf("execution scope has no schedulable resources")
	}
	router := newCompactPRISMRouter(request.NetworkTopology, resources)
	rankByID, err := prismCommunicationRanksWithRouter(request, topological, resources, router)
	if err != nil {
		return compactPRISMContext{}, err
	}
	ranked := append([]domain.Activity(nil), topological...)
	sort.SliceStable(ranked, func(i, j int) bool {
		left, right := rankByID[ranked[i].ID], rankByID[ranked[j].ID]
		if left != right {
			return left > right
		}
		return ranked[i].ID < ranked[j].ID
	})
	search := compactPRISMContext{
		request:          request,
		activities:       ranked,
		activityOrdinal:  make(map[string]int, len(ranked)),
		predecessors:     make([][]compactPRISMDependency, len(ranked)),
		successors:       make([][]int, len(ranked)),
		durations:        make([][]float64, len(ranked)),
		feasible:         make([][]bool, len(ranked)),
		minimumCosts:     make([]float64, len(ranked)),
		averageDurations: make([]float64, len(ranked)),
		ranks:            make([]float64, len(ranked)),
		router:           router,
		readyBranchLimit: intOption(configuration, "readyBranchLimit", 3, 1, 16),
		beamWidth:        intOption(configuration, "beamWidth", 120, 1, 10000),
	}
	for ordinal, activity := range ranked {
		search.activityOrdinal[activity.ID] = ordinal
		search.ranks[ordinal] = rankByID[activity.ID]
	}
	search.buildResources(resources)
	search.buildDependencies()
	search.buildExecutionMatrices()
	return search, nil
}

func (search *compactPRISMContext) buildResources(resources []domain.Resource) {
	search.resources = make([]compactPRISMResource, 0, len(resources))
	for _, resource := range resources {
		resourceCores := cores(resource)
		search.resources = append(search.resources, compactPRISMResource{
			resource:   resource,
			cores:      resourceCores,
			coreOffset: search.coreCount,
		})
		search.coreCount += len(resourceCores)
	}
}

func (search *compactPRISMContext) buildDependencies() {
	bytes := dataBytes(search.request.Workflow)
	for _, dependency := range search.request.Workflow.Dependencies {
		predecessor, predecessorExists := search.activityOrdinal[dependency.DependsOnActivityID]
		activity, activityExists := search.activityOrdinal[dependency.ActivityID]
		if !predecessorExists || !activityExists {
			continue
		}
		search.predecessors[activity] = append(
			search.predecessors[activity],
			compactPRISMDependency{
				predecessor: predecessor,
				bytes:       bytes[dependency.ActivityID][dependency.DependsOnActivityID],
			},
		)
		search.successors[predecessor] = append(search.successors[predecessor], activity)
	}
}

func (search *compactPRISMContext) buildExecutionMatrices() {
	resources := extractCompactPRISMResources(search.resources)
	for activityOrdinal, activity := range search.activities {
		search.durations[activityOrdinal] = make([]float64, len(search.resources))
		search.feasible[activityOrdinal] = make([]bool, len(search.resources))
		minimumCost := minimumActivityCost(
			activity,
			resources,
			search.request.ActivityProfiles,
		)
		search.minimumCosts[activityOrdinal] = minimumCost
		for resourceOrdinal, resource := range search.resources {
			search.feasible[activityOrdinal][resourceOrdinal] = resourceFeasible(
				activity,
				resource.resource,
			)
			search.durations[activityOrdinal][resourceOrdinal] = duration(
				activity,
				resource.resource,
				search.request.ActivityProfiles,
			)
			search.averageDurations[activityOrdinal] += search.durations[activityOrdinal][resourceOrdinal]
		}
		search.averageDurations[activityOrdinal] /= float64(len(search.resources))
	}
}

func extractCompactPRISMResources(resources []compactPRISMResource) []domain.Resource {
	items := make([]domain.Resource, len(resources))
	for index, resource := range resources {
		items[index] = resource.resource
	}
	return items
}
