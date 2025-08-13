package states

import (
	"context"
	"fmt"

	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_pb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_tpb"
	"github.com/robfig/cron/v3"
)

func NewTriggerStateMachine() (*trigger_pb.TriggerPSM, error) {
	config := trigger_pb.TriggerPSMBuilder()

	sm, err := config.BuildStateMachine()
	if err != nil {
		return nil, err
	}

	// CREATED -> ACTIVE
	sm.From(0).
		OnEvent(trigger_pb.TriggerPSMEventCreated).
		SetStatus(trigger_pb.TriggerStatus_ACTIVE).
		Mutate(trigger_pb.TriggerPSMMutation(func(
			state *trigger_pb.TriggerData,
			event *trigger_pb.TriggerEventType_Created,
		) error {
			err := validateCronString(event.Cron)
			if err != nil {
				return fmt.Errorf("update trigger: %w", err)
			}

			state.Cron = event.Cron
			state.AppName = event.AppName
			state.TriggerName = event.TriggerName
			state.RequestMetadata = event.RequestMetadata
			return nil
		}))

	// ACTIVE -> UPDATED
	sm.From(trigger_pb.TriggerStatus_ACTIVE).
		OnEvent(trigger_pb.TriggerPSMEventUpdated).
		SetStatus(trigger_pb.TriggerStatus_ACTIVE).
		Mutate(trigger_pb.TriggerPSMMutation(func(
			state *trigger_pb.TriggerData,
			event *trigger_pb.TriggerEventType_Updated,
		) error {
			err := validateCronString(event.Cron)
			if err != nil {
				return fmt.Errorf("update trigger: %w", err)
			}

			state.Cron = event.Cron
			state.AppName = event.AppName
			state.TriggerName = event.TriggerName
			state.RequestMetadata = event.RequestMetadata
			return nil
		}))

	// ACTIVE -> TRIGGERED
	sm.From(trigger_pb.TriggerStatus_ACTIVE).
		OnEvent(trigger_pb.TriggerPSMEventTriggered).
		LogicHook(trigger_pb.TriggerPSMLogicHook(func(
			ctx context.Context,
			tb trigger_pb.TriggerPSMHookBaton,
			state *trigger_pb.TriggerState,
			event *trigger_pb.TriggerEventType_Triggered,
		) error {

			reply := &trigger_tpb.TriggerReplyMessage{
				Request:  state.Data.RequestMetadata,
				TickTime: event.TriggerTime,
			}

			tb.SideEffect(reply)

			return nil
		}))

	// ACTIVE -> MANUALLY_TRIGGERED
	sm.From(trigger_pb.TriggerStatus_ACTIVE).
		OnEvent(trigger_pb.TriggerPSMEventManuallyTriggered).
		LogicHook(trigger_pb.TriggerPSMLogicHook(func(
			ctx context.Context,
			tb trigger_pb.TriggerPSMHookBaton,
			state *trigger_pb.TriggerState,
			event *trigger_pb.TriggerEventType_ManuallyTriggered,
		) error {

			reply := &trigger_tpb.TriggerReplyMessage{
				Request:  state.Data.RequestMetadata,
				TickTime: event.TriggerTime,
			}

			tb.SideEffect(reply)

			return nil
		}))

	// ACTIVE -> PAUSED
	sm.From(trigger_pb.TriggerStatus_ACTIVE).
		OnEvent(trigger_pb.TriggerPSMEventPaused).
		SetStatus(trigger_pb.TriggerStatus_PAUSED)

	// ACTIVE -> ARCHIVED
	sm.From(trigger_pb.TriggerStatus_ACTIVE).
		OnEvent(trigger_pb.TriggerPSMEventArchived).
		SetStatus(trigger_pb.TriggerStatus_ARCHIVED)

	// PAUSED -> ACTIVE
	sm.From(trigger_pb.TriggerStatus_PAUSED).
		OnEvent(trigger_pb.TriggerPSMEventActivated).
		SetStatus(trigger_pb.TriggerStatus_ACTIVE)

	// PAUSED -> PAUSED update event
	sm.From(trigger_pb.TriggerStatus_PAUSED).
		OnEvent(trigger_pb.TriggerPSMEventUpdated).
		SetStatus(trigger_pb.TriggerStatus_PAUSED).
		Mutate(trigger_pb.TriggerPSMMutation(func(
			state *trigger_pb.TriggerData,
			event *trigger_pb.TriggerEventType_Updated,
		) error {
			err := validateCronString(event.Cron)
			if err != nil {
				return fmt.Errorf("update trigger: %w", err)
			}

			state.Cron = event.Cron
			state.AppName = event.AppName
			state.TriggerName = event.TriggerName
			state.RequestMetadata = event.RequestMetadata
			return nil
		}))

	// PAUSED -> ARCHIVED
	sm.From(trigger_pb.TriggerStatus_PAUSED).
		OnEvent(trigger_pb.TriggerPSMEventArchived).
		SetStatus(trigger_pb.TriggerStatus_ARCHIVED)

	return sm, nil
}

func validateCronString(c string) error {
	_, err := cron.ParseStandard(c)
	if err != nil {
		return fmt.Errorf("invalid cron string: %w", err)
	}
	return nil
}
