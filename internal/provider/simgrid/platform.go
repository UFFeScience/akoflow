package simgrid

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type platformEdge struct {
	target    string
	linkIndex int
	weight    float64
}

func buildPlatformXML(resources []domain.Resource, topology domain.NetworkTopology, referenceFLOPS float64) ([]byte, error) {
	if len(resources) == 0 {
		return nil, fmt.Errorf("SimGrid platform requires at least one resource")
	}
	var output bytes.Buffer
	output.WriteString(`<?xml version="1.0"?>` + "\n")
	output.WriteString(`<!DOCTYPE platform SYSTEM "https://simgrid.org/simgrid.dtd">` + "\n")
	output.WriteString(`<platform version="4.1">` + "\n")
	output.WriteString(`  <zone id="akoflow" routing="Full">` + "\n")
	resourceIDs := make([]string, 0, len(resources))
	for _, resource := range resources {
		if resource.ID == "" {
			return nil, fmt.Errorf("SimGrid resource id is required")
		}
		cores := resource.CPUCores
		if cores < 1 {
			cores = 1
		}
		resourceIDs = append(resourceIDs, resource.ID)
		fmt.Fprintf(&output, "    <host id=\"%s\" speed=\"%.9ff\" core=\"%d\"/>\n",
			xmlEscape(resource.ID), resourceFLOPS(resource, referenceFLOPS), cores)
	}
	for index, link := range topology.Links {
		if link.BandwidthBitsPerSecond <= 0 {
			return nil, fmt.Errorf("SimGrid link %q requires positive bandwidth", link.ID)
		}
		fmt.Fprintf(&output,
			"    <link id=\"link-%d\" bandwidth=\"%.9fBps\" latency=\"%.9fs\" sharing_policy=\"%s\"/>\n",
			index, link.BandwidthBitsPerSecond/8, link.LatencySeconds, sharingPolicy(link.SharingPolicy))
	}
	sort.Strings(resourceIDs)
	graph := platformGraph(topology.Links)
	for _, source := range resourceIDs {
		for _, target := range resourceIDs {
			if source == target {
				continue
			}
			path, found := shortestLinkPath(graph, source, target)
			if !found {
				continue
			}
			fmt.Fprintf(&output, "    <route src=\"%s\" dst=\"%s\" symmetrical=\"NO\">\n",
				xmlEscape(source), xmlEscape(target))
			for _, linkIndex := range path {
				fmt.Fprintf(&output, "      <link_ctn id=\"link-%d\"/>\n", linkIndex)
			}
			output.WriteString("    </route>\n")
		}
	}
	output.WriteString("  </zone>\n</platform>\n")
	return output.Bytes(), nil
}

func platformGraph(links []domain.NetworkLink) map[string][]platformEdge {
	graph := make(map[string][]platformEdge)
	for index, link := range links {
		weight := link.LatencySeconds + 8/max(link.BandwidthBitsPerSecond, 1)
		graph[link.SourceResourceID] = append(graph[link.SourceResourceID], platformEdge{
			target: link.TargetResourceID, linkIndex: index, weight: weight,
		})
		if link.Bidirectional {
			graph[link.TargetResourceID] = append(graph[link.TargetResourceID], platformEdge{
				target: link.SourceResourceID, linkIndex: index, weight: weight,
			})
		}
	}
	return graph
}

func shortestLinkPath(graph map[string][]platformEdge, source, target string) ([]int, bool) {
	distance := map[string]float64{source: 0}
	paths := map[string][]int{source: nil}
	visited := make(map[string]bool)
	for {
		current := ""
		for node, value := range distance {
			if !visited[node] && (current == "" || value < distance[current]) {
				current = node
			}
		}
		if current == "" {
			return nil, false
		}
		if current == target {
			return paths[current], true
		}
		visited[current] = true
		for _, edge := range graph[current] {
			candidate := distance[current] + edge.weight
			previous, exists := distance[edge.target]
			if exists && candidate >= previous {
				continue
			}
			distance[edge.target] = candidate
			paths[edge.target] = append(append([]int(nil), paths[current]...), edge.linkIndex)
		}
	}
}

func sharingPolicy(value string) string {
	if value == "independent" || value == "fatpipe" {
		return "FATPIPE"
	}
	return "SHARED"
}

func xmlEscape(value string) string {
	var output bytes.Buffer
	_ = xml.EscapeText(&output, []byte(value))
	return output.String()
}
