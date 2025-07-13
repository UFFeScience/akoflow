package schedule_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/schedule_entity"
)

type IScheduleRepository interface {
	ListAllSchedules() ([]schedule_entity.ScheduleEntity, error)
	// GetScheduleById(id int) (schedule_entity.ScheduleEntity, error)
	CreateSchedule(name string, scheduleType string, code string, soFile string) (schedule_entity.ScheduleEntity, error)
	GetScheduleByName(name string) (schedule_entity.ScheduleEntity, error)
	// UpdateSchedule(schedule schedule_entity.ScheduleEntity) (schedule_entity.ScheduleEntity, error)
	// DeleteSchedule(id int) error
}

type ScheduleRepository struct {
	tableName string
}

var TableName = "schedules"

func New() IScheduleRepository {

	database := repository.Database{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, model.ScheduleModel{})
	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &ScheduleRepository{
		tableName: TableName,
	}
}

func (r *ScheduleRepository) ListAllSchedules() ([]schedule_entity.ScheduleEntity, error) {
	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT id, type, code, name, plugin_so_path, created_at, updated_at FROM " + r.tableName)
	if err != nil {
		return nil, err
	}

	var schedules []schedule_entity.ScheduleEntity
	for rows.Next() {
		var schedule model.ScheduleModel

		var pluginSo *string // Assuming plugin_so_path is a string, adjust if it's a different type

		err = rows.Scan(&schedule.ID, &schedule.Type, &schedule.Code, &schedule.Name, &pluginSo, &schedule.CreatedAt, &schedule.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// If pluginSo is nil, set it to an empty string
		if pluginSo == nil {
			schedule.PluginSoPath = ""
		} else {
			schedule.PluginSoPath = *pluginSo
		}

		schedules = append(schedules, schedule_entity.ScheduleEntity{
			ID:           schedule.ID,
			Type:         schedule.Type,
			Code:         schedule.Code,
			Name:         schedule.Name,
			PluginSoPath: schedule.PluginSoPath, // Uncomment if needed
			CreatedAt:    schedule.CreatedAt,    // Uncomment if needed
			UpdatedAt:    schedule.UpdatedAt,    // Uncomment if needed
		})
	}

	err = c.Close()
	if err != nil {
		return nil, err
	}

	return schedules, nil
}

func (r *ScheduleRepository) CreateSchedule(name string, scheduleType string, code string, soFile string) (schedule_entity.ScheduleEntity, error) {
	database := repository.Database{}
	c := database.Connect()

	schedule := model.ScheduleModel{
		Type: scheduleType,
		Code: code,
		Name: name,
	}

	query := "INSERT INTO " + r.tableName + " (type, code, name, plugin_so_path, created_at, updated_at) VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))"
	result, err := c.Exec(query, schedule.Type, schedule.Code, schedule.Name, soFile)
	if err != nil {
		return schedule_entity.ScheduleEntity{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return schedule_entity.ScheduleEntity{}, err
	}

	schedule.ID = int(id)

	err = c.Close()
	if err != nil {
		return schedule_entity.ScheduleEntity{}, err
	}

	return schedule_entity.ScheduleEntity{
		ID:           schedule.ID,
		Type:         schedule.Type,
		Code:         schedule.Code,
		Name:         schedule.Name,
		PluginSoPath: schedule.PluginSoPath,
		CreatedAt:    schedule.CreatedAt,
		UpdatedAt:    schedule.UpdatedAt,
	}, nil
}

func (r *ScheduleRepository) GetScheduleByName(name string) (schedule_entity.ScheduleEntity, error) {
	database := repository.Database{}
	c := database.Connect()

	query := "SELECT id, type, code, plugin_so_path, created_at, updated_at name FROM " + r.tableName + " WHERE name = ?"
	row := c.QueryRow(query, name)

	var schedule model.ScheduleModel
	err := row.Scan(&schedule.ID, &schedule.Type, &schedule.Code, &schedule.Name)
	if err != nil {
		return schedule_entity.ScheduleEntity{}, err
	}

	err = c.Close()
	if err != nil {
		return schedule_entity.ScheduleEntity{}, err
	}

	return schedule_entity.ScheduleEntity{
		ID:           schedule.ID,
		Type:         schedule.Type,
		Code:         schedule.Code,
		Name:         schedule.Name,
		PluginSoPath: schedule.PluginSoPath, // Uncomment if needed
		CreatedAt:    schedule.CreatedAt,    // Uncomment if needed
		UpdatedAt:    schedule.UpdatedAt,    // Uncomment if needed
	}, nil
}
