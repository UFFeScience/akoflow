package schedule_entity

type ScheduleEntity struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Code string `json:"code"`
	Name string `json:"name,omitempty"` // Optional field, not always present
}

func (s ScheduleEntity) GetId() int {
	return s.ID
}

func (s ScheduleEntity) GetType() string {
	return s.Type
}

func (s ScheduleEntity) GetCode() string {
	return s.Code
}

func (s ScheduleEntity) GetName() string {
	if s.Name == "" {
		return "default"
	}
	return s.Name
}

func New(schedule ScheduleEntity) ScheduleEntity {
	return ScheduleEntity{
		ID:   schedule.ID,
		Type: schedule.Type,
		Code: schedule.Code,
		Name: schedule.Name,
	}
}
