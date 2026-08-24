package main

import (
	"context"
	"database/sql"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	dbaudit "github.com/UFFeScience/akoflow/internal/infrastructure/database/audit"
	dbconsole "github.com/UFFeScience/akoflow/internal/infrastructure/database/console"
	dbdata "github.com/UFFeScience/akoflow/internal/infrastructure/database/data"
	dbenvironment "github.com/UFFeScience/akoflow/internal/infrastructure/database/environment"
	dbexecution "github.com/UFFeScience/akoflow/internal/infrastructure/database/execution"
	dbinstance "github.com/UFFeScience/akoflow/internal/infrastructure/database/instance"
	dbnetwork "github.com/UFFeScience/akoflow/internal/infrastructure/database/network"
	dbplanning "github.com/UFFeScience/akoflow/internal/infrastructure/database/planning"
	dbqueue "github.com/UFFeScience/akoflow/internal/infrastructure/database/queue"
	dbresource "github.com/UFFeScience/akoflow/internal/infrastructure/database/resource"
	dbstorage "github.com/UFFeScience/akoflow/internal/infrastructure/database/storage"
	dbworkflow "github.com/UFFeScience/akoflow/internal/infrastructure/database/workflow"
)

type persistence struct {
	database     *sql.DB
	environments *dbenvironment.Repository
	executions   *dbexecution.Repository
	data         *dbdata.Repository
	topologies   *dbnetwork.Repository
	plans        *dbplanning.Repository
	events       *dbqueue.Repository
	workflows    *dbworkflow.Repository
	resources    *dbresource.Repository
	instance     *dbinstance.Repository
	audit        *dbaudit.Repository
	console      *dbconsole.Repository
	storage      *dbstorage.Repository
}

func openPersistence(ctx context.Context) (persistence, error) {
	db, err := database.Open("")
	if err != nil {
		return persistence{}, err
	}
	if err := database.Bootstrap(ctx, db); err != nil {
		_ = db.Close()
		return persistence{}, err
	}
	events, err := dbqueue.New(db)
	if err != nil {
		_ = db.Close()
		return persistence{}, err
	}
	instanceRepository := dbinstance.New(db)
	if err := ensureSystemInstance(ctx, instanceRepository); err != nil {
		_ = db.Close()
		return persistence{}, err
	}
	return persistence{
		database: db, environments: dbenvironment.New(db), executions: dbexecution.New(db),
		data:       dbdata.New(db),
		topologies: dbnetwork.New(db), plans: dbplanning.New(db), events: events,
		workflows: dbworkflow.New(db),
		resources: dbresource.New(db),
		instance:  instanceRepository,
		audit:     dbaudit.New(db),
		console:   dbconsole.New(db),
		storage:   dbstorage.New(db),
	}, nil
}
