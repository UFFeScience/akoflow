package schedule_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/schedule_entity"
)

type IScheduleRepository interface {
	ListAllSchedules() ([]schedule_entity.ScheduleEntity, error)
	// GetScheduleById(id int) (schedule_entity.ScheduleEntity, error)
	CreateSchedule(name string, scheduleType string, code string) (schedule_entity.ScheduleEntity, error)
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

	rows, err := c.Query("SELECT id, type, code, name FROM " + r.tableName)
	if err != nil {
		return nil, err
	}

	var schedules []schedule_entity.ScheduleEntity
	for rows.Next() {
		var schedule model.ScheduleModel
		err = rows.Scan(&schedule.ID, &schedule.Type, &schedule.Code, &schedule.Name)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule_entity.ScheduleEntity{
			ID:   schedule.ID,
			Type: schedule.Type,
			Code: schedule.Code,
			Name: schedule.Name,
		})
	}

	err = c.Close()
	if err != nil {
		return nil, err
	}

	return schedules, nil
}

func (r *ScheduleRepository) CreateSchedule(name string, scheduleType string, code string) (schedule_entity.ScheduleEntity, error) {
	database := repository.Database{}
	c := database.Connect()

	schedule := model.ScheduleModel{
		Type: scheduleType,
		Code: code,
		Name: name,
	}

	query := "INSERT INTO " + r.tableName + " (type, code, name) VALUES (?, ?, ?)"
	result, err := c.Exec(query, schedule.Type, schedule.Code, schedule.Name)
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
		ID:   schedule.ID,
		Type: schedule.Type,
		Code: schedule.Code,
		Name: schedule.Name,
	}, nil
}

func (r *ScheduleRepository) GetScheduleByName(name string) (schedule_entity.ScheduleEntity, error) {
	database := repository.Database{}
	c := database.Connect()

	query := "SELECT id, type, code, name FROM " + r.tableName + " WHERE name = ?"
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
		ID:   schedule.ID,
		Type: schedule.Type,
		Code: schedule.Code,
		Name: schedule.Name,
	}, nil
}
