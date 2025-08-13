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

func TestManualTrigger(tt *testing.T) {
	flow, uu := NewUniverse(tt)
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

		trmsg := &trigger_tpb.TriggerReplyMessage{}
		uu.Outbox.PopMessage(t, trmsg)
		t.Equal(TriggerTime, trmsg.TickTime)
	})
}
