package core

// BuildProcessTree builds a hierarchical tree from a flat list of process records
// Processes with PPID=0 or PPID not found in the list are treated as root processes
func BuildProcessTree(records []ResourceRecord) []*ProcessTreeNode {
	if len(records) == 0 {
		return []*ProcessTreeNode{}
	}

	// Build lookup maps
	nodeMap := make(map[int32]*ProcessTreeNode)    // PID -> Node
	pidExists := make(map[int32]bool)              // Track which PIDs exist

	// First pass: Create all nodes and track existing PIDs
	for _, record := range records {
		node := &ProcessTreeNode{
			Process:    record,
			Children:   []*ProcessTreeNode{},
			IsExpanded: true, // Default expanded for UI
		}
		nodeMap[record.PID] = node
		pidExists[record.PID] = true
	}

	// Second pass: Build parent-child relationships
	var rootNodes []*ProcessTreeNode
	for _, node := range nodeMap {
		ppid := node.Process.PPID
		
		// Treat as root if:
		// - PPID is 0 (no parent)
		// - PPID is 1 (init/systemd - but this is filtered out)
		// - Parent doesn't exist in our list (orphaned)
		if ppid == 0 || ppid == 1 || !pidExists[ppid] {
			rootNodes = append(rootNodes, node)
		} else {
			// Add to parent's children
			if parent, exists := nodeMap[ppid]; exists {
				parent.Children = append(parent.Children, node)
			} else {
				// Parent not found, treat as root
				rootNodes = append(rootNodes, node)
			}
		}
	}

	// Third pass: Calculate aggregates recursively
	for _, root := range rootNodes {
		calculateTreeAggregates(root)
	}

	return rootNodes
}

// calculateTreeAggregates recursively calculates total CPU and memory for a tree node
// TotalCPU and TotalMemory include the process itself and all its descendants
func calculateTreeAggregates(node *ProcessTreeNode) {
	// Start with self
	node.TotalCPU = node.Process.CPUPercentNormalized
	node.TotalMemory = node.Process.MemoryMB
	node.ChildCount = len(node.Children)

	// Recursively add children
	for _, child := range node.Children {
		calculateTreeAggregates(child)
		node.TotalCPU += child.TotalCPU
		node.TotalMemory += child.TotalMemory
		node.ChildCount += child.ChildCount // Include grandchildren in count
	}
}

// FlattenTree converts a tree structure back to a flat list
// Useful for backward compatibility or alternative views
func FlattenTree(nodes []*ProcessTreeNode) []ResourceRecord {
	var records []ResourceRecord
	var flatten func(*ProcessTreeNode)
	
	flatten = func(node *ProcessTreeNode) {
		records = append(records, node.Process)
		for _, child := range node.Children {
			flatten(child)
		}
	}

	for _, node := range nodes {
		flatten(node)
	}

	return records
}

// GetProcessGroupSummary creates a summary of process groups
// Groups processes by name and returns aggregated statistics
func GetProcessGroupSummary(records []ResourceRecord) map[string]*ProcessTreeNode {
	groups := make(map[string]*ProcessTreeNode)

	for _, record := range records {
		name := record.Name
		if group, exists := groups[name]; exists {
			// Add to existing group
			group.TotalCPU += record.CPUPercentNormalized
			group.TotalMemory += record.MemoryMB
			group.ChildCount++
		} else {
			// Create new group
			groups[name] = &ProcessTreeNode{
				Process:     record,
				Children:    []*ProcessTreeNode{},
				TotalCPU:    record.CPUPercentNormalized,
				TotalMemory: record.MemoryMB,
				ChildCount:  1,
				IsExpanded:  false,
			}
		}
	}

	return groups
}
