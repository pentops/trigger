package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pentops/flowtest"
	"github.com/pentops/j5/gen/j5/messaging/v1/messaging_j5pb"
	"github.com/pentops/j5/lib/id62"
	"github.com/pentops/o5-auth/authtest"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_pb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_spb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_tpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTickRequest(tt *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flow, uu := NewUniverse(ctx, tt)
	defer flow.RunSteps(tt)

	triggerID := id62.NewString()

	flow.Step("create trigger", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		req := &trigger_tpb.TickRequestMessage{
			Request: &messaging_j5pb.RequestMetadata{
				Context: []byte(""),
			},
			Action: &trigger_pb.ActionType{
				Type: &trigger_pb.ActionType_Create_{
					Create: &trigger_pb.ActionType_Create{
						TriggerId:   proto.String(triggerID),
						AppName:     "test",
						TriggerName: "TestCron",
						Cron:        "CRON_TZ=America/New_York 0 7 * * *",
					},
				},
			},
		}

		_, err := uu.TriggerWorker.TickRequest(ctx, req)
		t.NoError(err)

		resp, err := uu.Query.TriggerGet(ctx, &trigger_spb.TriggerGetRequest{TriggerId: triggerID})
		t.NoError(err)
		t.NotNil(resp)
		t.Equal("ACTIVE", resp.Trigger.Status.ShortString())
		t.Equal("CRON_TZ=America/New_York 0 7 * * *", resp.Trigger.Data.Cron)
	})

	flow.Step("update trigger", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		req := &trigger_tpb.TickRequestMessage{
			Request: &messaging_j5pb.RequestMetadata{
				Context: []byte(""),
			},
			Action: &trigger_pb.ActionType{
				Type: &trigger_pb.ActionType_Update_{
					Update: &trigger_pb.ActionType_Update{
						TriggerId:   triggerID,
						AppName:     "test",
						TriggerName: "TestCron",
						Cron:        "CRON_TZ=America/New_York 0 8 * * *",
					},
				},
			},
		}

		_, err := uu.TriggerWorker.TickRequest(ctx, req)
		t.NoError(err)

		resp, err := uu.Query.TriggerGet(ctx, &trigger_spb.TriggerGetRequest{TriggerId: triggerID})
		t.NoError(err)
		t.NotNil(resp)
		t.Equal("ACTIVE", resp.Trigger.Status.ShortString())
		t.Equal("CRON_TZ=America/New_York 0 8 * * *", resp.Trigger.Data.Cron)
	})

	flow.Step("archive trigger", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		req := &trigger_tpb.TickRequestMessage{
			Request: &messaging_j5pb.RequestMetadata{
				Context: []byte(""),
			},
			Action: &trigger_pb.ActionType{
				Type: &trigger_pb.ActionType_Archive_{
					Archive: &trigger_pb.ActionType_Archive{
						TriggerId: triggerID,
					},
				},
			},
		}

		_, err := uu.TriggerWorker.TickRequest(ctx, req)
		t.NoError(err)

		resp, err := uu.Query.TriggerGet(ctx, &trigger_spb.TriggerGetRequest{TriggerId: triggerID})
		t.NoError(err)
		t.NotNil(resp)
		t.Equal("ARCHIVED", resp.Trigger.Status.ShortString())
	})
}

func TestActive(tt *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flow, uu := NewUniverse(ctx, tt)
	defer flow.RunSteps(tt)

	TriggerID := id62.NewString()

	flow.Step("create trigger", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		err := uu.CreateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID,
			AppName:     "test",
			TriggerName: "TestMonthlyCron",
			Cron:        "0 18 1 * *",
		})
		t.NoError(err)

		resp, err := uu.Query.TriggerGet(ctx, &trigger_spb.TriggerGetRequest{
			TriggerId: TriggerID,
		})
		t.NoError(err)
		t.NotNil(resp)
		t.Equal("ACTIVE", resp.Trigger.Status.ShortString())
		t.Equal("TestMonthlyCron", resp.Trigger.Data.TriggerName)
		t.Equal("0 18 1 * *", resp.Trigger.Data.Cron)
	})

	flow.Step("update trigger", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		err := uu.UpdateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID,
			AppName:     "test",
			TriggerName: "TestMonthlyCron",
			Cron:        "0 20 * * *",
		})
		t.NoError(err)

		resp, err := uu.Query.TriggerGet(ctx, &trigger_spb.TriggerGetRequest{
			TriggerId: TriggerID,
		})
		t.NoError(err)
		t.NotNil(resp)
		t.Equal("ACTIVE", resp.Trigger.Status.ShortString())
		t.Equal("TestMonthlyCron", resp.Trigger.Data.TriggerName)
		t.Equal("0 20 * * *", resp.Trigger.Data.Cron)
	})

	flow.Step("archive trigger", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		err := uu.ArchiveTrigger(ctx, TriggerID)
		t.NoError(err)

		resp, err := uu.Query.TriggerGet(ctx, &trigger_spb.TriggerGetRequest{
			TriggerId: TriggerID,
		})
		t.NoError(err)
		t.NotNil(resp)
		t.Equal("ARCHIVED", resp.Trigger.Status.ShortString())
	})
}

func TestPause(tt *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flow, uu := NewUniverse(ctx, tt)
	defer flow.RunSteps(tt)

	TriggerID := id62.NewString()

	flow.Step("create trigger", func(ctx context.Context, t flowtest.Asserter) {
		err := uu.CreateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID,
			AppName:     "test",
			TriggerName: "TestMonthlyCron",
			Cron:        "0 18 1 * *",
		})
		t.NoError(err)

		resp, err := uu.Query.TriggerGet(ctx, &trigger_spb.TriggerGetRequest{
			TriggerId: TriggerID,
		})
		t.NoError(err)
		t.NotNil(resp)
		t.Equal("ACTIVE", resp.Trigger.Status.ShortString())
		t.Equal("TestMonthlyCron", resp.Trigger.Data.TriggerName)
		t.Equal("0 18 1 * *", resp.Trigger.Data.Cron)
	})

	flow.Step("update trigger while active", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		err := uu.UpdateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID,
			AppName:     "testUpdated",
			TriggerName: "TestMonthlyCronUpdated",
			Cron:        "0 20 1 * *",
		})
		t.NoError(err)

		resp, err := uu.Query.TriggerGet(ctx, &trigger_spb.TriggerGetRequest{
			TriggerId: TriggerID,
		})

		t.NoError(err)
		t.NotNil(resp)
		t.Equal("ACTIVE", resp.Trigger.Status.ShortString())
		t.Equal("TestMonthlyCronUpdated", resp.Trigger.Data.TriggerName)
		t.Equal("0 20 1 * *", resp.Trigger.Data.Cron)
	})

	flow.Step("pause trigger in prep for archive", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		pauseResp, err := uu.TriggerCommand.PauseTrigger(ctx, &trigger_spb.PauseTriggerRequest{
			TriggerId: TriggerID,
		})
		t.NoError(err)
		t.NotNil(pauseResp)
		t.Equal("PAUSED", pauseResp.Trigger.Status.ShortString())
	})

	flow.Step("update trigger while paused", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		err := uu.UpdateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID,
			AppName:     "testUpdated",
			TriggerName: "TestMonthlyCronPaused",
			Cron:        "0 19 1 * *",
		})
		t.NoError(err)

		resp, err := uu.Query.TriggerGet(ctx, &trigger_spb.TriggerGetRequest{
			TriggerId: TriggerID,
		})

		t.NoError(err)
		t.NotNil(resp)
		t.Equal("PAUSED", resp.Trigger.Status.ShortString())
		t.Equal("TestMonthlyCronPaused", resp.Trigger.Data.TriggerName)
		t.Equal("0 19 1 * *", resp.Trigger.Data.Cron)
	})

	flow.Step("resume paused trigger", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		pauseResp, err := uu.TriggerCommand.ResumeTrigger(ctx, &trigger_spb.ResumeTriggerRequest{
			TriggerId: TriggerID,
		})
		t.NoError(err)
		t.NotNil(pauseResp)
		t.Equal("ACTIVE", pauseResp.Trigger.Status.ShortString())
	})

	flow.Step("archive paused trigger", func(ctx context.Context, t flowtest.Asserter) {
		err := uu.ArchiveTrigger(ctx, TriggerID)
		t.NoError(err)

		resp, err := uu.Query.TriggerGet(ctx, &trigger_spb.TriggerGetRequest{
			TriggerId: TriggerID,
		})
		t.NoError(err)
		t.NotNil(resp)
		t.Equal("ARCHIVED", resp.Trigger.Status.ShortString())
	})
}

func TestManualTrigger(tt *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flow, uu := NewUniverse(ctx, tt)
	defer flow.RunSteps(tt)

	TriggerID := id62.NewString()
	TriggerTime := timestamppb.New(time.Now())

	flow.Step("create trigger", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		err := uu.CreateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID,
			AppName:     "test",
			TriggerName: "TestCron",
			Cron:        "0 18 1 * *",
		})
		t.NoError(err)
	})

	flow.Step("manually trigger", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		r, err := uu.TriggerCommand.ManuallyTrigger(ctx, &trigger_spb.ManuallyTriggerRequest{
			TriggerId:   TriggerID,
			TriggerTime: TriggerTime,
		})
		t.NoError(err)
		t.NotNil(r)
		t.Equal("ACTIVE", r.Trigger.Status.ShortString())

		resp, err := uu.Query.TriggerEvents(ctx, &trigger_spb.TriggerEventsRequest{
			TriggerId: TriggerID,
		})
		t.NoError(err)
		t.NotNil(resp)
		t.NotEmpty(resp.Events[0].Event.GetManuallyTriggered()) // Most recent should be a manually triggered event
		t.Equal(TriggerTime, resp.Events[0].Event.GetManuallyTriggered().TriggerTime)

		trmsg := &trigger_tpb.TickReplyMessage{}
		uu.Outbox.PopMessage(t, trmsg)
		t.Equal(TriggerTime, trmsg.TickTime)
	})
}

func TestGrumpyTrigger(tt *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flow, uu := NewUniverse(ctx, tt)
	defer flow.RunSteps(tt)

	TriggerID := id62.NewString()

	flow.Step("create trigger", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		err := uu.CreateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID,
			AppName:     "test",
			TriggerName: "TestCron",
			Cron:        "fail",
		})
		t.NotNil(err)
		t.Equal(true, strings.Contains(err.Error(), "invalid cron string: expected exactly 5 fields"))

		err = uu.CreateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID,
			AppName:     "test",
			TriggerName: "TestCron",
			Cron:        "0 18 1 * *",
		})
		t.NoError(err)

		err = uu.UpdateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID,
			AppName:     "test",
			TriggerName: "TestCron",
			Cron:        "99 99 * * *",
		})

		t.Equal(true, strings.Contains(err.Error(), "invalid cron string: end of range"))

		err = uu.UpdateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID,
			AppName:     "test",
			TriggerName: "TestCron",
			Cron:        "0 20 1 * *",
		})
		t.NoError(err)
	})
}
