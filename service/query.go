package service

import (
	"context"

	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_pb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_spb"

	"github.com/pentops/protostate/pquery"
	"github.com/pentops/protostate/psm"
	"google.golang.org/grpc"
)

type QueryService struct {
	db pquery.Transactor

	triggerQuery *trigger_spb.TriggerPSMQuerySet
	trigger_spb.UnimplementedTriggerQueryServiceServer
}

func NewQueryService(db pquery.Transactor,
	triggerSM *trigger_pb.TriggerPSM,
) (*QueryService, error) {

	triggerQuery, err := trigger_spb.NewTriggerPSMQuerySet(trigger_spb.DefaultTriggerPSMQuerySpec(triggerSM.StateTableSpec()), psm.StateQueryOptions{})
	if err != nil {
		return nil, err
	}

	return &QueryService{
		db:           db,
		triggerQuery: triggerQuery,
	}, nil
}

func (qs *QueryService) RegisterGRPC(s grpc.ServiceRegistrar) {
	trigger_spb.RegisterTriggerQueryServiceServer(s, qs)
}

func (s *QueryService) TriggerGet(ctx context.Context, req *trigger_spb.TriggerGetRequest) (*trigger_spb.TriggerGetResponse, error) {
	resObject := &trigger_spb.TriggerGetResponse{}
	err := s.triggerQuery.Get(ctx, s.db, req, resObject)
	if err != nil {
		return nil, err
	}
	return resObject, nil
}

func (s *QueryService) TriggerList(ctx context.Context, req *trigger_spb.TriggerListRequest) (*trigger_spb.TriggerListResponse, error) {
	resObject := &trigger_spb.TriggerListResponse{}
	err := s.triggerQuery.List(ctx, s.db, req, resObject)
	if err != nil {
		return nil, err
	}
	return resObject, nil
}

func (s *QueryService) TriggerEvents(ctx context.Context, req *trigger_spb.TriggerEventsRequest) (*trigger_spb.TriggerEventsResponse, error) {
	resObject := &trigger_spb.TriggerEventsResponse{}
	err := s.triggerQuery.ListEvents(ctx, s.db, req, resObject)
	if err != nil {
		return nil, err
	}
	return resObject, nil
}
