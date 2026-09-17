package main

import (
	"context"
	"database/sql"
	"errors"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	dbaudit "github.com/UFFeScience/akoflow/internal/infrastructure/database/audit"
	dbcloud "github.com/UFFeScience/akoflow/internal/infrastructure/database/cloud"
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
	"github.com/UFFeScience/akoflow/internal/infrastructure/instancearchive"
)

type persistence struct {
	database     *sql.DB
	analytics    *sql.DB
	readOnly     bool
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
	cloud        *dbcloud.Repository
}

func openPersistence(ctx context.Context, recreateOnSchemaChange bool) (persistence, error) {
	readOnly := instancearchive.IsReadOnlySelection()
	path := instancearchive.ResolveDatabasePath()
	if !readOnly {
		if _, err := database.ApplyPendingFactoryReset(path); err != nil {
			return persistence{}, err
		}
	}
	var db *sql.DB
	var err error
	if readOnly {
		db, err = database.OpenReadOnly(path)
	} else {
		db, err = database.Open(path)
	}
	if err != nil {
		return persistence{}, err
	}
	if !readOnly {
		if err := database.Bootstrap(ctx, db); err != nil {
			_ = db.Close()
			if !recreateOnSchemaChange || !errors.Is(err, database.ErrIncompatibleSchema) {
				return persistence{}, err
			}
			if err := database.Recreate(path); err != nil {
				return persistence{}, err
			}
			db, err = database.Open(path)
			if err != nil {
				return persistence{}, err
			}
			if err := database.Bootstrap(ctx, db); err != nil {
				_ = db.Close()
				return persistence{}, err
			}
		}
	}
	analytics, err := database.OpenReadOnly(path)
	if err != nil {
		_ = db.Close()
		return persistence{}, err
	}
	events, err := dbqueue.New(db)
	if err != nil {
		_ = analytics.Close()
		_ = db.Close()
		return persistence{}, err
	}
	instanceRepository := dbinstance.New(db)
	cloudRepository := dbcloud.New(db)
	executionRepository := dbexecution.New(db)
	if !readOnly {
		if err := executionRepository.EnsureMetricSchema(ctx); err != nil {
			_ = analytics.Close()
			_ = db.Close()
			return persistence{}, err
		}
		if err := ensureSystemInstance(ctx, instanceRepository); err != nil {
			_ = analytics.Close()
			_ = db.Close()
			return persistence{}, err
		}
		if err := cloudRepository.EnsureDefaults(ctx); err != nil {
			_ = analytics.Close()
			_ = db.Close()
			return persistence{}, err
		}
	}
	return persistence{
		database: db, analytics: analytics, readOnly: readOnly,
		environments: dbenvironment.New(db), executions: executionRepository,
		data:       dbdata.New(db),
		topologies: dbnetwork.New(db), plans: dbplanning.New(db), events: events,
		workflows: dbworkflow.New(db),
		resources: dbresource.New(db),
		instance:  instanceRepository,
		audit:     dbaudit.New(db),
		console:   dbconsole.New(db),
		storage:   dbstorage.New(db),
		cloud:     cloudRepository,
	}, nil
}
