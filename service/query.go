package service

import (
	"github.com/pentops/j5/lib/psm"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_spb"

	"github.com/pentops/j5/lib/j5query"
	"google.golang.org/grpc"
)

type QueryService struct {
	triggerQuery *trigger_spb.TriggerQueryServiceImpl
}

func NewQueryService(db j5query.Transactor,
	triggerSpec trigger_spb.TriggerPSMQuerySpec,
) (*QueryService, error) {

	triggerQuery, err := trigger_spb.NewTriggerPSMQuerySet(
		triggerSpec,
		psm.StateQueryOptions{},
	)
	if err != nil {
		return nil, err
	}

	return &QueryService{
		triggerQuery: trigger_spb.NewTriggerQueryServiceImpl(db, triggerQuery),
	}, nil
}

func (qs *QueryService) RegisterGRPC(s grpc.ServiceRegistrar) {
	trigger_spb.RegisterTriggerQueryServiceServer(s, qs.triggerQuery)
}
