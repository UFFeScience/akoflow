package node_repository

import (
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
)

type NodeRepository struct {
	tableName string
}

const STATUS_READY = 1
const STATUS_NOT_READY = 0

type INodeRepository interface {
	CreateOrUpdate(runtime string, node model.Node) error
	GetByName(name string) (*model.Node, error)
	UpdateNode(node model.Node) error
	GetNodesByRuntime(runtime string) ([]model.Node, error)
}

func New() INodeRepository {

	database := repository.Database{}
	model := model.Node{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, model)
	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &NodeRepository{
		tableName: model.TableName(),
	}
}

func (nr *NodeRepository) GetByName(name string) (*model.Node, error) {
	database := repository.Database{}
	c := database.Connect()
	defer c.Close()

	var node model.Node
	cols := node.GetColumns()
	rows, err := c.Query("SELECT "+strings.Join(cols, ", ")+" FROM "+nr.tableName+" WHERE name = ?", name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		err = rows.Scan(
			&node.Name,
			&node.Runtime,
			&node.Status,
			&node.CPUUsage,
			&node.CPUMax,
			&node.MemoryUsage,
			&node.MemoryLimit,
			&node.NetworkLimit,
			&node.NetworkUsage,
			&node.CreatedAt,
			&node.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}
		return &node, nil
	}

	return nil, nil // Node not found

}

func (nr *NodeRepository) UpdateNode(node model.Node) error {
	database := repository.Database{}
	c := database.Connect()
	defer c.Close()

	// Prepare the update statement
	stmt, err := c.Prepare("UPDATE " + nr.tableName + " SET status = ?, cpu_usage = ?, cpu_max = ?, memory_usage = ?, memory_limit = ?, network_limit = ?, network_usage = ?, updated_at = datetime('now') WHERE name = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Execute the update statement
	_, err = stmt.Exec(node.Status, node.CPUUsage, node.CPUMax, node.MemoryUsage, node.MemoryLimit, node.NetworkLimit, node.NetworkUsage, node.Name)
	if err != nil {
		return err
	}

	return nil
}

// CreateOrUpdate creates or updates a node in the database.
// If the node already exists, it updates the existing record.
// If the node does not exist, it creates a new record.
// The function takes the node name, status, and metadata as parameters.
func (nr *NodeRepository) CreateOrUpdate(runtime string, node model.Node) error {
	database := repository.Database{}
	c := database.Connect()
	defer c.Close()

	existingNode, err := nr.GetByName(node.Name)
	if err != nil {
		return err
	}

	if existingNode != nil {
		// Node exists, update it
		node.Name = existingNode.Name // Ensure we update the correct record
		return nr.UpdateNode(node)
	}

	// Node does not exist, create it
	stmt, err := c.Prepare("INSERT INTO " + nr.tableName + " (name, runtime, status, cpu_usage, cpu_max, memory_usage, memory_limit, network_limit, network_usage, created_at, updated_at) VALUES (?, ?, ?,  ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(node.Name, node.Runtime, node.Status, node.CPUUsage, node.CPUMax, node.MemoryUsage, node.MemoryLimit, node.NetworkLimit, node.NetworkUsage)
	if err != nil {
		return err
	}

	return nil

}

func (nr *NodeRepository) GetNodesByRuntime(runtime string) ([]model.Node, error) {
	database := repository.Database{}
	c := database.Connect()
	defer c.Close()

	var nodes []model.Node
	cols := model.Node{}.GetColumns()
	rows, err := c.Query("SELECT "+strings.Join(cols, ", ")+" FROM "+nr.tableName+" WHERE runtime = ?", runtime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var node model.Node
		err = rows.Scan(
			&node.Name,
			&node.Runtime,
			&node.Status,
			&node.CPUUsage,
			&node.CPUMax,
			&node.MemoryUsage,
			&node.MemoryLimit,
			&node.NetworkLimit,
			&node.NetworkUsage,
			&node.CreatedAt,
			&node.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}
