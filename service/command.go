package service

import (
	"context"

	"github.com/pentops/log.go/log"
	"github.com/pentops/realms/j5auth"
	"github.com/pentops/sqrlx.go/sqrlx"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_pb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_spb"
	"github.com/pentops/trigger/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TriggerCommand struct {
	db sqrlx.Transactor
	sm *trigger_pb.TriggerPSM

	trigger_spb.UnimplementedTriggerCommandServiceServer
}

func NewTriggerCommand(db sqrlx.Transactor, sm *trigger_pb.TriggerPSM) (*TriggerCommand, error) {
	return &TriggerCommand{
		db: db,
		sm: sm,
	}, nil
}

func (w *TriggerCommand) PauseTrigger(ctx context.Context, req *trigger_spb.PauseTriggerRequest) (*trigger_spb.PauseTriggerResponse, error) {
	action, err := j5auth.GetAuthenticatedAction(ctx)
	if err != nil {
		log.WithError(ctx, err).Error("failed get authenticated action in pause trigger")
		return nil, status.Error(codes.NotFound, "")
	}

	evt := trigger_pb.TriggerPSMEventSpec{
		Keys: &trigger_pb.TriggerKeys{
			TriggerId: req.TriggerId,
		},
		Action: action,
		Event:  &trigger_pb.TriggerEventType_Paused{},
	}

	resp := &trigger_spb.PauseTriggerResponse{}

	err = w.db.Transact(ctx, utils.MutableTxOptions, func(ctx context.Context, tx sqrlx.Transaction) error {
		triggerState, err := w.sm.TransitionInTx(ctx, tx, &evt)
		if err != nil {
			log.WithError(ctx, err).Error("failed to pause trigger")
			return status.Error(codes.Internal, "failed to pause trigger")
		}
		resp.Trigger = triggerState

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (w *TriggerCommand) ResumeTrigger(ctx context.Context, req *trigger_spb.ResumeTriggerRequest) (*trigger_spb.ResumeTriggerResponse, error) {
	action, err := j5auth.GetAuthenticatedAction(ctx)
	if err != nil {
		log.WithError(ctx, err).Error("failed get authenticated action in resume trigger")
		return nil, status.Error(codes.NotFound, "")
	}

	evt := trigger_pb.TriggerPSMEventSpec{
		Keys: &trigger_pb.TriggerKeys{
			TriggerId: req.TriggerId,
		},
		Action: action,
		Event:  &trigger_pb.TriggerEventType_Activated{},
	}

	resp := &trigger_spb.ResumeTriggerResponse{}

	err = w.db.Transact(ctx, utils.MutableTxOptions, func(ctx context.Context, tx sqrlx.Transaction) error {
		triggerState, err := w.sm.TransitionInTx(ctx, tx, &evt)
		if err != nil {
			log.WithError(ctx, err).Error("failed to resume trigger")
			return status.Error(codes.Internal, "failed to resume trigger")
		}
		resp.Trigger = triggerState

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (w *TriggerCommand) ManuallyTrigger(ctx context.Context, req *trigger_spb.ManuallyTriggerRequest) (*trigger_spb.ManuallyTriggerResponse, error) {
	action, err := j5auth.GetAuthenticatedAction(ctx)
	if err != nil {
		log.WithError(ctx, err).Error("failed get authenticated action in manual trigger")
		return nil, status.Error(codes.NotFound, "")
	}

	evt := trigger_pb.TriggerPSMEventSpec{
		Keys: &trigger_pb.TriggerKeys{
			TriggerId: req.TriggerId,
		},
		Action: action,
		Event: &trigger_pb.TriggerEventType_ManuallyTriggered{
			TriggerTime: req.TriggerTime,
		},
	}

	resp := &trigger_spb.ManuallyTriggerResponse{}

	err = w.db.Transact(ctx, utils.MutableTxOptions, func(ctx context.Context, tx sqrlx.Transaction) error {
		triggerState, err := w.sm.TransitionInTx(ctx, tx, &evt)
		if err != nil {
			log.WithError(ctx, err).Error("failed to manually trigger")
			return status.Error(codes.Internal, "failed to manually trigger")
		}
		resp.Trigger = triggerState

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}
