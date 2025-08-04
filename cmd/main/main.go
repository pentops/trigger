package main

import (
	"context"
	"fmt"
	"os"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	_ "github.com/lib/pq"
	"github.com/pentops/grpc.go/grpcbind"
	"github.com/pentops/j5/lib/j5grpc"
	"github.com/pentops/j5/lib/psm/psmigrate"
	"github.com/pentops/runner/commander"
	"github.com/pentops/sqrlx.go/pgenv"
	"github.com/pentops/sqrlx.go/sqrlx"
	"github.com/pentops/trigger/service"
	"github.com/pentops/trigger/states"
	"github.com/pressly/goose"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var Version string

func main() {
	mainGroup := commander.NewCommandSet()

	mainGroup.Add("serve", commander.NewCommand(runServe))
	mainGroup.Add("migrate", commander.NewCommand(runMigrate))
	mainGroup.Add("psm-tables", commander.NewCommand(runPSMTables))
	mainGroup.Add("info", commander.NewCommand(runServiceInfo))

	mainGroup.RunMain("trigger", Version)
}

func runPSMTables(ctx context.Context, cfg struct{}) error {
	sm, err := states.NewTriggerStateMachine()
	if err != nil {
		return err
	}

	migrationFile, err := psmigrate.BuildStateMachineMigrations(sm.StateTableSpec())
	if err != nil {
		return fmt.Errorf("build migration file: %w", err)
	}

	fmt.Println(string(migrationFile))
	return nil
}

func runMigrate(ctx context.Context, config struct {
	MigrationsDir string `env:"MIGRATIONS_DIR" default:"./ext/db"`
	pgenv.DatabaseConfig
}) error {

	db, err := config.OpenPostgres(ctx)
	if err != nil {
		return err
	}

	return goose.Up(db, config.MigrationsDir)
}

func runServiceInfo(ctx context.Context, config struct {
}) error {

	grpcServer := grpc.NewServer()
	if err := buildAndRegister(ctx, nil, grpcServer); err != nil {
		return err
	}

	return j5grpc.PrintServerInfo(os.Stdout, grpcServer)
}

func runServe(ctx context.Context, config struct {
	grpcbind.EnvConfig
	pgenv.DatabaseConfig
}) error {

	db, err := config.OpenPostgresTransactor(ctx)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		service.GRPCUnaryMiddleware(Version, false)...,
	)))

	if err := buildAndRegister(ctx, db, grpcServer); err != nil {
		return err
	}
	reflection.Register(grpcServer)

	return config.ListenAndServe(ctx, grpcServer)

}

func buildAndRegister(ctx context.Context, db sqrlx.Transactor, grpcServer grpc.ServiceRegistrar) error {
	serviceSet, err := service.BuildService(db)
	if err != nil {
		return fmt.Errorf("failed to build service: %w", err)
	}
	serviceSet.RegisterGRPC(grpcServer)
	err = serviceSet.TriggerWorker.InitSelfTick(ctx)
	if err != nil {
		return fmt.Errorf("failed to init self tick: %w", err)
	}

	return nil

}
