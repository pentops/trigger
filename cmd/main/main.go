package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"sort"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	_ "github.com/lib/pq"
	"github.com/pentops/j5/gen/j5/messaging/v1/messaging_j5pb"
	"github.com/pentops/log.go/log"
	"github.com/pentops/protostate/psmigrate"
	"github.com/pentops/runner/commander"
	"github.com/pentops/sqrlx.go/sqrlx"
	"github.com/pentops/trigger/service"
	"github.com/pentops/trigger/states"
	"github.com/pressly/goose"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
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

func openDatabase(ctx context.Context, dbURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	// Default is unlimited connections, use a cap to prevent hammering the database if it's the bottleneck.
	// 10 was selected as a conservative number and will likely be revised later.
	db.SetMaxOpenConns(10)

	for {
		if err := db.Ping(); err != nil {
			log.WithError(ctx, err).Error("pinging PG")
			time.Sleep(time.Second)
			continue
		}
		break
	}

	return db, nil
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
	PostgresURL   string `env:"POSTGRES_URL"`
}) error {

	db, err := openDatabase(ctx, config.PostgresURL)
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

	info := grpcServer.GetServiceInfo()

	subscriptions := make([]string, 0)

	paths := make([]string, 0)

	for name := range info {
		desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
		if err != nil {
			return err
		}

		serviceDesc, ok := desc.(protoreflect.ServiceDescriptor)
		if !ok {
			return fmt.Errorf("not a service: %s", name)
		}

		serviceOpt := proto.GetExtension(desc.Options(), messaging_j5pb.E_Service).(*messaging_j5pb.ServiceConfig)
		if serviceOpt != nil {
			var role string
			switch serviceOpt.Role.(type) {
			case *messaging_j5pb.ServiceConfig_Publish_:
				role = "publish"
			case *messaging_j5pb.ServiceConfig_Request_:
				role = "request"
			case *messaging_j5pb.ServiceConfig_Reply_:
				role = "reply"
			}

			var topic string
			if serviceOpt.TopicName != nil {
				topic = *serviceOpt.TopicName
			}

			subscriptions = append(subscriptions, fmt.Sprintf("  - name: \"/%s\" # %s as %s", name, role, topic))
			continue
		}

		for i := 0; i < serviceDesc.Methods().Len(); i++ {
			method := serviceDesc.Methods().Get(i)
			fmt.Printf("  %s\n", method.FullName())

			httpOpt := proto.GetExtension(method.Options(), annotations.E_Http).(*annotations.HttpRule)
			if httpOpt == nil {
				return fmt.Errorf("no http rule on %s", method.FullName())
			}

			var httpMethod string
			var httpPath string

			switch pt := httpOpt.Pattern.(type) {
			case *annotations.HttpRule_Get:
				httpMethod = "GET"
				httpPath = pt.Get
			case *annotations.HttpRule_Post:
				httpMethod = "POST"
				httpPath = pt.Post
			case *annotations.HttpRule_Put:
				httpMethod = "PUT"
				httpPath = pt.Put
			case *annotations.HttpRule_Delete:
				httpMethod = "DELETE"
				httpPath = pt.Delete
			case *annotations.HttpRule_Patch:
				httpMethod = "PATCH"
				httpPath = pt.Patch

			default:
				return fmt.Errorf("unsupported http method %T", pt)
			}

			paths = append(paths, fmt.Sprintf("%s %s", httpPath, httpMethod))
		}

	}

	sort.Strings(paths)
	for _, path := range paths {
		fmt.Println(path)
	}

	sort.Strings(subscriptions)
	fmt.Println("subscriptions:")
	for _, sub := range subscriptions {
		fmt.Println(sub)
	}

	return nil
}

func runServe(ctx context.Context, config struct {
	PublicAddr  string `env:"PUBLIC_ADDR" default:":8081"`
	PostgresURL string `env:"POSTGRES_URL"`
}) error {

	dbConn, err := openDatabase(ctx, config.PostgresURL)
	if err != nil {
		return err
	}

	db := sqrlx.NewPostgres(dbConn)

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		service.GRPCUnaryMiddleware(Version, false)...,
	)))

	if err := buildAndRegister(ctx, db, grpcServer); err != nil {
		return err
	}
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", config.PublicAddr)
	if err != nil {
		return err
	}
	log.WithField(ctx, "addr", lis.Addr().String()).Info("Begin Server")
	closeOnContextCancel(ctx, grpcServer)

	return grpcServer.Serve(lis)
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

func closeOnContextCancel(ctx context.Context, srv *grpc.Server) {
	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()
}
