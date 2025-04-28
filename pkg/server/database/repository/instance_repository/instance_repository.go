package instance_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
)

type InstanceRepository struct {
}
type IInstanceRepository interface {
}

func New() IInstanceRepository {

	database := repository.Database{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, model.Instance{})
	if err != nil {
		return nil
	}
	c.Close()

	c = database.Connect()
	err = repository.CreateOrVerifyTable(c, model.ActivityDependency{})
	if err != nil {
		return nil
	}

	c.Close()

	c = database.Connect()
	err = repository.CreateOrVerifyTable(c, model.PreActivity{})

	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &InstanceRepository{}
}
