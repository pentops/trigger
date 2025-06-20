package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/elgris/sqrl"
	"github.com/pentops/j5/gen/j5/state/v1/psm_j5pb"
	"github.com/pentops/j5/lib/id62"
	"github.com/pentops/log.go/log"
	"github.com/pentops/o5-messaging/outbox"
	"github.com/pentops/sqrlx.go/sqrlx"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_pb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_tpb"
	"github.com/pentops/trigger/utils"
	"github.com/robfig/cron/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TriggerWorker struct {
	db     sqrlx.Transactor
	sm     *trigger_pb.TriggerPSM
	sender *outbox.Sender

	trigger_tpb.UnimplementedTriggerPublishTopicServer
	trigger_tpb.UnimplementedSelfTickTopicServer
	trigger_tpb.UnimplementedTickRequestTopicServer
	trigger_tpb.UnimplementedTickReplyTopicServer
}

const (
	triggerCron    = "*/1 * * * *" // every 1 minutes
	triggerCadence = 1 * time.Minute
)

var ErrNotFound = errors.New("not found")

func NewTriggerWorker(db sqrlx.Transactor, sm *trigger_pb.TriggerPSM) (*TriggerWorker, error) {
	sender := outbox.NewSender(outbox.DefaultConfig)

	return &TriggerWorker{
		db:     db,
		sender: sender,
		sm:     sm,
	}, nil
}

func (w TriggerWorker) InitSelfTick(ctx context.Context) error {
	_, err := w.GetLastTick(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			log.Info(ctx, "no previous self tick found, creating one")
			now := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour(), time.Now().Minute()-1, 0, 0, time.UTC)
			err = w.SendSelfTick(ctx, &now)
			if err != nil {
				return fmt.Errorf("failed to sendSelfTick during InitSelfTick: %v", err)
			}
			return nil
		}
		return fmt.Errorf("failed to InitSelfTick %v", err)
	}

	return nil
}

func (w *TriggerWorker) TickRequest(ctx context.Context, req *trigger_tpb.TickRequestMessage) (*emptypb.Empty, error) {
	var evt *trigger_pb.TriggerPSMEventSpec

	switch req.Action.Type.(type) {
	case *trigger_pb.ActionType_Create_:
		if validateCron(req.Action.GetCreate().Cron) != nil {
			return nil, fmt.Errorf("invalid cron string: %v", validateCron(req.Action.GetCreate().Cron).Error())
		}

		newTriggerID := id62.NewString()
		triggerIDFromAction := req.GetAction().GetCreate().TriggerId
		if triggerIDFromAction != "" {
			newTriggerID = triggerIDFromAction
		}

		evt = &trigger_pb.TriggerPSMEventSpec{
			Keys: &trigger_pb.TriggerKeys{
				TriggerId: newTriggerID,
			},
			Cause: &psm_j5pb.Cause{
				Type: &psm_j5pb.Cause_ExternalEvent{
					ExternalEvent: &psm_j5pb.ExternalEventCause{
						SystemName: "trigger",
						EventName:  "trigger_create",
					},
				},
			},
			Event: &trigger_pb.TriggerEventType_Created{
				TriggerName:     req.Action.GetCreate().TriggerName,
				AppName:         req.Action.GetCreate().AppName,
				Cron:            req.Action.GetCreate().Cron,
				RequestMetadata: req.GetJ5RequestMetadata(),
			},
		}

	case *trigger_pb.ActionType_Update_:
		if validateCron(req.Action.GetUpdate().Cron) != nil {
			return nil, fmt.Errorf("invalid cron string: %v", validateCron(req.Action.GetUpdate().Cron).Error())
		}

		evt = &trigger_pb.TriggerPSMEventSpec{
			Keys: &trigger_pb.TriggerKeys{
				TriggerId: req.Action.GetUpdate().TriggerId,
			},
			Cause: &psm_j5pb.Cause{
				Type: &psm_j5pb.Cause_ExternalEvent{
					ExternalEvent: &psm_j5pb.ExternalEventCause{
						SystemName: "trigger",
						EventName:  "trigger_update",
					},
				},
			},
			Event: &trigger_pb.TriggerEventType_Updated{
				TriggerName:     req.Action.GetUpdate().TriggerName,
				AppName:         req.Action.GetUpdate().AppName,
				Cron:            req.Action.GetUpdate().Cron,
				RequestMetadata: req.GetJ5RequestMetadata(),
			},
		}

	case *trigger_pb.ActionType_Archive_:
		evt = &trigger_pb.TriggerPSMEventSpec{
			Keys: &trigger_pb.TriggerKeys{
				TriggerId: req.Action.GetArchive().TriggerId,
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

	default:
		return nil, fmt.Errorf("unknown action type %T", req.Action.Type)
	}

	_, err := w.sm.Transition(ctx, w.db, evt)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (w *TriggerWorker) SelfTick(ctx context.Context, req *trigger_tpb.SelfTickMessage) (*emptypb.Empty, error) {
	activeTriggers, err := w.AllActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("self tick: %w", err)
	}

	triggerTime, err := nextTick(req.LastTick.AsTime())
	if err != nil {
		return nil, fmt.Errorf("failed to get trigger time %v", err)
	}

	for _, trigger := range activeTriggers {
		sendTriggerEvt, err := checkCron(trigger.Data.Cron, *triggerTime)
		if err != nil {
			return nil, err
		}

		if sendTriggerEvt {
			evt := trigger_pb.TriggerPSMEventSpec{
				Keys: &trigger_pb.TriggerKeys{
					TriggerId: trigger.Keys.TriggerId,
				},
				Cause: &psm_j5pb.Cause{
					Type: &psm_j5pb.Cause_ExternalEvent{
						ExternalEvent: &psm_j5pb.ExternalEventCause{
							SystemName: "trigger",
							EventName:  "trigger_tick",
						},
					},
				},
				Event: &trigger_pb.TriggerEventType_Triggered{
					TriggerTime: timestamppb.New(*triggerTime),
				},
			}

			err := w.db.Transact(ctx, utils.MutableTxOptions, func(ctx context.Context, tx sqrlx.Transaction) error {
				_, err := w.sm.TransitionInTx(ctx, tx, &evt)
				if err != nil {
					return fmt.Errorf("failed to trigger event: %w", err)
				}

				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}

	err = w.SendSelfTick(ctx, triggerTime)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (w TriggerWorker) SendSelfTick(ctx context.Context, triggeredTime *time.Time) error {
	err := w.db.Transact(ctx, utils.MutableTxOptions, func(ctx context.Context, tx sqrlx.Transaction) error {
		// send self tick
		msg := &trigger_tpb.SelfTickMessage{
			LastTick: timestamppb.New(*triggeredTime),
		}

		delay := calcDelay(*triggeredTime, time.Now().In(time.UTC))
		err := w.sender.SendDelayed(ctx, tx, delay, msg)
		if err != nil {
			return fmt.Errorf("failed to send delayed tick %v", err)
		}

		// upsert selftick table
		asJSON, err := protojson.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal SelfTickMessage: %w", err)
		}

		query := sqrlx.Upsert("selftick").
			Key("selftick_id", "selftick").
			Set("data", asJSON).
			Set("lasttick", triggeredTime).
			Where("EXCLUDED.lasttick > selftick.lasttick")

		_, err = tx.Insert(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to upsert selftick %v", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (w TriggerWorker) AllActive(ctx context.Context) ([]*trigger_pb.TriggerState, error) {
	var triggers []*trigger_pb.TriggerState

	if err := w.db.Transact(ctx, utils.ReadOnlyTxOptions, func(ctx context.Context, tx sqrlx.Transaction) error {
		rows, err := tx.Query(
			ctx,
			sq.Select("state").
				From("trigger").
				Where("state->>'status' = 'TRIGGER_STATUS_ACTIVE'"),
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("error while getting active triggers %v", err)
		}

		defer rows.Close()

		for rows.Next() {
			var data []byte
			if err := rows.Scan(&data); err != nil {
				return fmt.Errorf("failed to scan active trigger row %v", err)
			}
			tm := &trigger_pb.TriggerState{}
			if err := protojson.Unmarshal(data, tm); err != nil {
				return fmt.Errorf("failed to unmarshal trigger state %v", err)
			}

			triggers = append(triggers, tm)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return triggers, nil
}

func (w TriggerWorker) GetLastTick(ctx context.Context) (*trigger_tpb.SelfTickMessage, error) {
	query := sq.Select("data").
		From("selftick").
		Where("selftick_id = ?", "selftick")

	var data []byte
	if err := w.db.Transact(ctx, utils.ReadOnlyTxOptions, func(ctx context.Context, tx sqrlx.Transaction) error {
		data = []byte{}
		return tx.QueryRow(ctx, query).Scan(&data)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	msg := &trigger_tpb.SelfTickMessage{}
	if err := protojson.Unmarshal(data, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func checkCron(c string, thisTick time.Time) (bool, error) {
	sched, err := cron.ParseStandard(c)
	if err != nil {
		return false, fmt.Errorf("failed to parse cron string %v", err)
	}

	lastTick := thisTick.Add(triggerCadence * -1)
	// cannot compare current time with the cron, so go back one cadence,
	// get the next which is current, then compare.
	return sched.Next(lastTick).Equal(thisTick), nil

}

func nextTick(lastTick time.Time) (*time.Time, error) {
	sched, err := cron.ParseStandard(triggerCron)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cron string %v", err)
	}
	nextTick := sched.Next(lastTick)
	return &nextTick, nil
}

// calcDelay calculates the delay for the next tick based on the this tick time and the time passed.
// The max delay is 5 minutes. If the next tick is in the past, it returns 0.
func calcDelay(thisTick, now time.Time) time.Duration {
	nextTick := thisTick.Add(triggerCadence)

	// next tick should have already happened, return 0
	if nextTick.Before(now) {
		return 0
	}

	return min(nextTick.Sub(now), 5*time.Minute)
}

func validateCron(c string) error {
	_, err := cron.ParseStandard(c)
	if err != nil {
		return fmt.Errorf("invalid cron string: %w", err)
	}
	return nil
}
