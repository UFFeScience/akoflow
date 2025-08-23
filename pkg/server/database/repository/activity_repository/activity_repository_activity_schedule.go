package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
)

func (w *ActivityRepository) SetActivitySchedule(workflowId int, activityId int, nodeName string, scheduleName string, cpuRequired float64, memoryRequired float64, metadata string) error {
	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec(
		"INSERT INTO "+w.tableNameActivitySchedule+" (workflow_id, activity_id, node_name, schedule_name, cpu_required, memory_required, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))",
		workflowId,
		activityId,
		nodeName,
		scheduleName,
		cpuRequired,
		memoryRequired,
		metadata,
	)

	if err != nil {
		return err
	}

	err = c.Close()
	if err != nil {
		return err
	}

	return nil
}

func (w *ActivityRepository) GetActivityScheduleByNodeName(nodeName string) ([]model.ActivitySchedule, error) {
	database := repository.Database{}
	c := database.Connect()
	defer c.Close()

	var activitySchedules []model.ActivitySchedule
	rows, err := c.Query("SELECT id, workflow_id, activity_id, node_name, schedule_name, cpu_required, memory_required, metadata, created_at FROM "+w.tableNameActivitySchedule+" WHERE node_name = ?", nodeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var activitySchedule model.ActivitySchedule
		err = rows.Scan(
			&activitySchedule.ID,
			&activitySchedule.WorkflowID,
			&activitySchedule.ActivityID,
			&activitySchedule.NodeName,
			&activitySchedule.ScheduleName,
			&activitySchedule.CpuRequired,
			&activitySchedule.MemoryRequired,
			&activitySchedule.Metadata,
			&activitySchedule.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		activitySchedules = append(activitySchedules, activitySchedule)
	}

	return activitySchedules, nil
}

func (w *ActivityRepository) GetActivityScheduleByActivityId(activityId int) (model.ActivitySchedule, error) {
	database := repository.Database{}
	c := database.Connect()
	defer c.Close()

	var activitySchedules []model.ActivitySchedule
	rows, err := c.Query("SELECT id, workflow_id, activity_id, node_name, schedule_name, cpu_required, memory_required, metadata, created_at FROM "+w.tableNameActivitySchedule+" WHERE activity_id = ?", activityId)
	if err != nil {
		return model.ActivitySchedule{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var activitySchedule model.ActivitySchedule
		err = rows.Scan(
			&activitySchedule.ID,
			&activitySchedule.WorkflowID,
			&activitySchedule.ActivityID,
			&activitySchedule.NodeName,
			&activitySchedule.ScheduleName,
			&activitySchedule.CpuRequired,
			&activitySchedule.MemoryRequired,
			&activitySchedule.Metadata,
			&activitySchedule.CreatedAt,
		)
		if err != nil {
			return model.ActivitySchedule{}, err
		}
		activitySchedules = append(activitySchedules, activitySchedule)
	}
	if len(activitySchedules) == 0 {
		return model.ActivitySchedule{}, nil // No activity schedule found for the given activity ID
	}
	return activitySchedules[0], nil // Return the first activity schedule found
}

func (w *ActivityRepository) IsActivityScheduled(workflowId int, activityId int) (bool, error) {
	database := repository.Database{}
	c := database.Connect()
	defer c.Close()

	var count int
	err := c.QueryRow("SELECT COUNT(*) FROM "+w.tableNameActivitySchedule+" WHERE workflow_id = ? AND activity_id = ?", workflowId, activityId).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
