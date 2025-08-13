package service

import (
	"fmt"

	"github.com/pentops/sqrlx.go/sqrlx"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_pb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_spb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_tpb"
	"github.com/pentops/trigger/states"
	"google.golang.org/grpc"
)

type Service struct {
	SM             *trigger_pb.TriggerPSM
	QueryService   *QueryService
	TriggerWorker  *TriggerWorker
	TriggerCommand *TriggerCommand
}

func BuildService(db sqrlx.Transactor) (*Service, error) {
	sm, err := states.NewTriggerStateMachine()
	if err != nil {
		return nil, err
	}

	query, err := NewQueryService(db, trigger_spb.DefaultTriggerPSMQuerySpec(sm.StateTableSpec()))
	if err != nil {
		return nil, fmt.Errorf("BuildService NewQueryService: %w", err)
	}

	triggerWorker, err := NewTriggerWorker(db, sm)
	if err != nil {
		return nil, fmt.Errorf("BuildService NewTriggerWorker: %w", err)
	}

	triggerCommand, err := NewTriggerCommand(db, sm)
	if err != nil {
		return nil, fmt.Errorf("BuildService NewTriggerCommand: %w", err)
	}

	return &Service{
		SM:             sm,
		QueryService:   query,
		TriggerWorker:  triggerWorker,
		TriggerCommand: triggerCommand,
	}, nil
}

func (a *Service) RegisterGRPC(server grpc.ServiceRegistrar) {
	a.QueryService.RegisterGRPC(server)
	trigger_spb.RegisterTriggerCommandServiceServer(server, a.TriggerCommand)
	trigger_tpb.RegisterTriggerPublishTopicServer(server, a.TriggerWorker)
	trigger_tpb.RegisterSelfTickTopicServer(server, a.TriggerWorker)
	trigger_tpb.RegisterTriggerManageRequestTopicServer(server, a.TriggerWorker)
	trigger_tpb.RegisterTriggerManageReplyTopicServer(server, a.TriggerWorker)
}
