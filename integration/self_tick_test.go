package integration

import (
	"context"
	"testing"
	"time"

	"github.com/pentops/flowtest"
	"github.com/pentops/j5/lib/id62"
	"github.com/pentops/o5-auth/authtest"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_spb"
	"github.com/pentops/trigger/gen/o5/trigger/v1/trigger_tpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSelfTick(tt *testing.T) {
	flow, uu := NewUniverse(tt)
	defer flow.RunSteps(tt)

	TriggerID1 := id62.NewString()
	TriggerID2 := id62.NewString()
	TriggerID3 := id62.NewString()
	TriggerID4 := id62.NewString()

	lastTickString := "2025-02-17 18:29:00Z"
	layout := "2006-01-02 15:04:05Z" // Format string for parsing

	lastTick, err := time.Parse(layout, lastTickString)
	if err != nil {
		tt.Fatal(err)
	}

	flow.Step("create triggers", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		err := uu.CreateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID1,
			AppName:     "test",
			TriggerName: "testCron1",
			Cron:        "29 18 * * *",
		})
		t.NoError(err)

		err = uu.CreateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID2,
			AppName:     "test",
			TriggerName: "testCron2",
			Cron:        "30 18 * * *",
		})
		t.NoError(err)

		err = uu.CreateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID3,
			AppName:     "test",
			TriggerName: "testCron3",
			Cron:        "31 18 * * *",
		})
		t.NoError(err)

		err = uu.CreateTrigger(ctx, triggerConfig{
			TriggerID:   TriggerID4,
			AppName:     "test",
			TriggerName: "testCron4",
			Cron:        "30 18 * * *",
		})
		t.NoError(err)
		_, err = uu.TriggerCommand.PauseTrigger(ctx, &trigger_spb.PauseTriggerRequest{
			TriggerId: TriggerID4,
		})
		t.NoError(err)
	})

	flow.Step("send self tick", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		_, err := uu.TickTopic.SelfTick(ctx, &trigger_tpb.SelfTickMessage{
			LastTick: timestamppb.New(lastTick),
		})
		t.NoError(err)

		stmsg := &trigger_tpb.SelfTickMessage{}
		uu.Outbox.PopMessage(t, stmsg)
		t.Equal(true, stmsg.LastTick.AsTime().Equal(lastTick.Add(1*time.Minute)))

		trmsg := &trigger_tpb.TriggerManageReplyMessage{}

		// last tick was sent, so need to add 1 minute to make sure the tick happened at the correct time.
		thisTick := lastTick.Add(1 * time.Minute)
		uu.Outbox.PopMessage(t, trmsg)
		t.Equal(thisTick, trmsg.TickTime.AsTime())
	})

	flow.Step("only testTrigger2 should have triggered", func(ctx context.Context, t flowtest.Asserter) {
		ctx = authtest.JWTContext(ctx)

		resp, err := uu.Query.TriggerEvents(ctx, &trigger_spb.TriggerEventsRequest{
			TriggerId: TriggerID2,
		})
		t.NoError(err)
		t.NotNil(resp)

		t.NotEmpty(resp.Events[0].Event.GetTriggered())
		resp, err = uu.Query.TriggerEvents(ctx, &trigger_spb.TriggerEventsRequest{
			TriggerId: TriggerID1,
		})
		t.NoError(err)
		t.NotNil(resp)

		t.Nil(resp.Events[0].Event.GetTriggered())

		resp, err = uu.Query.TriggerEvents(ctx, &trigger_spb.TriggerEventsRequest{
			TriggerId: TriggerID3,
		})
		t.NoError(err)
		t.NotNil(resp)

		t.Nil(resp.Events[0].Event.GetTriggered())

		resp, err = uu.Query.TriggerEvents(ctx, &trigger_spb.TriggerEventsRequest{
			TriggerId: TriggerID4,
		})
		t.NoError(err)
		t.NotNil(resp)

		t.Nil(resp.Events[0].Event.GetTriggered())
	})
}

func TestInitSelfTick(tt *testing.T) {
	flow, uu := NewUniverse(tt)
	defer flow.RunSteps(tt)

	flow.Step("init test", func(ctx context.Context, t flowtest.Asserter) {
		uu.mustTruncateSelfTable(ctx, t)

		ctx = authtest.JWTContext(ctx)
		err := uu.TriggerWorker.InitSelfTick(ctx)
		t.NoError(err)

		tick, err := uu.TriggerWorker.GetLastTick(ctx)
		t.NoError(err)
		t.NotNil(tick)

		stmsg := &trigger_tpb.SelfTickMessage{}
		uu.Outbox.PopMessage(t, stmsg)
	})
}
