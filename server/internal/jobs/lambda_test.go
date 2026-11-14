package jobs

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestWorkerRoutesEvents(t *testing.T) {
	var classified uuid.UUID
	swept := false
	w := Worker{
		Classify: func(_ context.Context, id uuid.UUID) error { classified = id; return nil },
		Sweep:    func(context.Context) (int, error) { swept = true; return 0, nil },
	}
	id := uuid.New()

	var ev WorkerEvent
	if err := json.Unmarshal([]byte(`{"itemId":"`+id.String()+`"}`), &ev); err != nil {
		t.Fatal(err)
	}
	if err := w.Handle(context.Background(), ev); err != nil || classified != id || swept {
		t.Fatalf("item event: err=%v classified=%v swept=%v", err, classified, swept)
	}

	classified = uuid.Nil
	if err := json.Unmarshal([]byte(`{"sweep":true}`), &ev); err != nil {
		t.Fatal(err)
	}
	ev.ItemID = uuid.Nil
	if err := w.Handle(context.Background(), ev); err != nil || !swept || classified != uuid.Nil {
		t.Fatalf("sweep event: err=%v classified=%v swept=%v", err, classified, swept)
	}

	if err := w.Handle(context.Background(), WorkerEvent{}); err == nil {
		t.Fatal("empty event: want an error")
	}
}

func TestInvokePayloadShape(t *testing.T) {
	id := uuid.New()
	b, _ := json.Marshal(WorkerEvent{ItemID: id})
	if string(b) != `{"itemId":"`+id.String()+`"}` {
		t.Fatalf("payload = %s", b)
	}
	b, _ = json.Marshal(WorkerEvent{Sweep: true})
	if string(b) != `{"itemId":"00000000-0000-0000-0000-000000000000","sweep":true}` && string(b) != `{"sweep":true}` {
		t.Fatalf("sweep payload = %s", b)
	}
}
