package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/pentops/flowtest"
	"github.com/pentops/j5/gen/j5/messaging/v1/messaging_j5pb"
	"github.com/pentops/j5/gen/j5/state/v1/psm_j5pb"
	"github.com/pentops/j5/lib/id62"
	"github.com/pentops/log.go/log"
	"github.com/pentops/o5-messaging/outbox/outboxtest"
	"github.com/pentops/pgtest.go/pgtest"
	"github.com/pentops/sqrlx.go/sqrlx"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_pb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_spb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_tpb"
	"github.com/pentops/trigger/service"
	"github.com/pentops/trigger/utils"
)

type Universe struct {
	db *sqrlx.Wrapper

	SM             *trigger_pb.TriggerPSM
	Query          trigger_spb.TriggerQueryServiceClient
	TriggerTopic   trigger_tpb.TriggerPublishTopicClient
	TriggerCommand trigger_spb.TriggerCommandServiceClient
	TickTopic      trigger_tpb.SelfTickTopicClient
	TriggerWorker  *service.TriggerWorker

	Outbox *outboxtest.OutboxAsserter
}

func NewUniverse(ctx context.Context, t *testing.T) (*flowtest.Stepper[*testing.T], *Universe) {
	name := t.Name()
	stepper := flowtest.NewStepper[*testing.T](name)
	uu := &Universe{}

	stepper.Setup(func(ctx context.Context, t flowtest.Asserter) error {
		log.DefaultLogger = log.NewCallbackLogger(stepper.LevelLog)
		setupUniverse(ctx, t, uu)
		return nil
	})

	stepper.PostStepHook(func(ctx context.Context, t flowtest.Asserter) error {
		uu.Outbox.AssertEmpty(t)
		return nil
	})

	return stepper, uu
}

func setupUniverse(ctx context.Context, t flowtest.Asserter, uu *Universe) {
	t.Helper()

	conn := pgtest.GetTestDB(t, pgtest.WithDir("../ext/db"))

	uu.Outbox = outboxtest.NewOutboxAsserter(t, conn)

	middleware := service.GRPCUnaryMiddleware("testing", false)

	grpcPair := flowtest.NewGRPCPair(t, middleware...)

	db := sqrlx.NewPostgres(conn)
	uu.db = db

	svc, err := service.BuildService(uu.db)
	if err != nil {
		t.Fatal(err)
	}

	uu.SM = svc.SM
	uu.Query = trigger_spb.NewTriggerQueryServiceClient(grpcPair.Client)
	uu.TriggerTopic = trigger_tpb.NewTriggerPublishTopicClient(grpcPair.Client)
	uu.TriggerCommand = trigger_spb.NewTriggerCommandServiceClient(grpcPair.Client)
	uu.TickTopic = trigger_tpb.NewSelfTickTopicClient(grpcPair.Client)
	uu.TriggerWorker = svc.TriggerWorker

	svc.RegisterGRPC(grpcPair.Server)

	grpcPair.ServeUntilDone(t, ctx)
}

func (uu *Universe) SendTriggerEvent(ctx context.Context, evt *trigger_pb.TriggerPSMEventSpec) error {
	err := uu.db.Transact(ctx, utils.MutableTxOptions, func(ctx context.Context, tx sqrlx.Transaction) error {
		_, err := uu.SM.TransitionInTx(ctx, tx, evt)
		if err != nil {
			return fmt.Errorf("failed to universe transition send trigger event: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to universe send trigger event: %w", err)
	}

	return nil
}

type triggerConfig struct {
	TriggerID       string
	TriggerName     string
	AppName         string
	Cron            string
	RequestMetadata *messaging_j5pb.RequestMetadata
}

func (uu *Universe) CreateTrigger(ctx context.Context, config triggerConfig) error {
	requestMetadata := &messaging_j5pb.RequestMetadata{
		ReplyTo: "test",
		Context: []byte("testContext"),
	}
	if config.RequestMetadata != nil {
		requestMetadata = config.RequestMetadata
	}

	appName := "testApp"
	if config.AppName != "" {
		appName = config.AppName
	}

	triggerName := "testTrigger"
	if config.TriggerName != "" {
		triggerName = config.TriggerName
	}

	cron := "0 * * * *"
	if config.Cron != "" {
		cron = config.Cron
	}

	var generatedTriggerID = id62.New().String()
	var triggerID = &generatedTriggerID

	if config.TriggerID != "" {
		triggerID = &config.TriggerID
	}

	req := &trigger_tpb.TickRequestMessage{
		Request: requestMetadata,
		Action: &trigger_pb.ActionType{
			Type: &trigger_pb.ActionType_Create_{
				Create: &trigger_pb.ActionType_Create{
					TriggerId:   *triggerID,
					TriggerName: triggerName,
					AppName:     appName,
					Cron:        cron,
				},
			},
		},
	}

	_, err := uu.TriggerWorker.TickRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to universe create trigger: %w", err)
	}

	return nil
}

func (uu *Universe) UpdateTrigger(ctx context.Context, config triggerConfig) error {
	requestMetadata := &messaging_j5pb.RequestMetadata{
		ReplyTo: "test",
		Context: []byte("testContext"),
	}
	if config.RequestMetadata != nil {
		requestMetadata = config.RequestMetadata
	}

	appName := "testApp"
	if config.AppName != "" {
		appName = config.AppName
	}

	triggerName := "testTrigger"
	if config.TriggerName != "" {
		triggerName = config.TriggerName
	}

	cron := "0 * * * *"
	if config.Cron != "" {
		cron = config.Cron
	}

	triggerID := id62.New().String()
	if config.TriggerID != "" {
		triggerID = config.TriggerID
	}

	req := &trigger_tpb.TickRequestMessage{
		Request: requestMetadata,
		Action: &trigger_pb.ActionType{
			Type: &trigger_pb.ActionType_Update_{
				Update: &trigger_pb.ActionType_Update{
					TriggerId:   triggerID,
					TriggerName: triggerName,
					AppName:     appName,
					Cron:        cron,
				},
			},
		},
	}

	_, err := uu.TriggerWorker.TickRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to universe update trigger: %w", err)
	}

	return nil
}

func (uu *Universe) ArchiveTrigger(ctx context.Context, triggerID string) error {
	evt := &trigger_pb.TriggerPSMEventSpec{
		Keys: &trigger_pb.TriggerKeys{
			TriggerId: triggerID,
		},
		Cause: &psm_j5pb.Cause{
			Type: &psm_j5pb.Cause_ExternalEvent{
				ExternalEvent: &psm_j5pb.ExternalEventCause{
					SystemName: "trigger",
					EventName:  "trigger_archive",
				},
			},
		},
		Event: &trigger_pb.TriggerEventType_Archived{},
	}

	err := uu.SendTriggerEvent(ctx, evt)
	if err != nil {
		return fmt.Errorf("failed to universe archive trigger: %w", err)
	}

	return nil
}

func (uu *Universe) mustTruncateSelfTable(ctx context.Context, t flowtest.Asserter) {
	err := uu.db.Transact(ctx, utils.MutableTxOptions, func(ctx context.Context, tx sqrlx.Transaction) error {
		_, err := tx.ExecRaw(ctx, "truncate table selftick")
		if err != nil {
			t.Fatalf("failed to truncate self tick table: %v", err)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("failed mustTruncateSelfTable: %v", err)
	}
}
