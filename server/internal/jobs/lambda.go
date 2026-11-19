package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/google/uuid"
)

// On AWS there is no River: the API invokes the worker Lambda asynchronously
// for each item (Lambda queues the event and retries it twice on failure),
// and a schedule invokes it with {"sweep": true} to catch anything missed.
// There's no queue service to pay for and nothing polling while idle.

// WorkerEvent is the worker Lambda's input.
type WorkerEvent struct {
	// ItemID asks for one item to be classified.
	ItemID uuid.UUID `json:"itemId,omitzero"`
	// PreviewItemID asks for one item's link preview to be fetched.
	PreviewItemID uuid.UUID `json:"previewItemId,omitzero"`
	// Sweep runs the catch-up pass instead.
	Sweep bool `json:"sweep,omitzero"`
}

// LambdaInvoker enqueues by invoking the worker Lambda asynchronously.
type LambdaInvoker struct {
	client       *lambda.Client
	functionName string
}

// NewLambdaInvoker uses the Lambda's own credentials and region.
func NewLambdaInvoker(ctx context.Context, functionName string) (*LambdaInvoker, error) {
	if functionName == "" {
		return nil, errors.New("jobs: WORKER_FUNCTION_NAME is not set")
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("jobs: AWS config: %w", err)
	}
	return &LambdaInvoker{client: lambda.NewFromConfig(cfg), functionName: functionName}, nil
}

// EnqueueClassify implements api.Enqueuer.
func (l *LambdaInvoker) EnqueueClassify(ctx context.Context, itemID uuid.UUID) error {
	return l.invoke(ctx, WorkerEvent{ItemID: itemID})
}

// EnqueuePreview implements api.Enqueuer.
func (l *LambdaInvoker) EnqueuePreview(ctx context.Context, itemID uuid.UUID) error {
	return l.invoke(ctx, WorkerEvent{PreviewItemID: itemID})
}

func (l *LambdaInvoker) invoke(ctx context.Context, ev WorkerEvent) error {
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = l.client.Invoke(ctx, &lambda.InvokeInput{
		FunctionName:   aws.String(l.functionName),
		InvocationType: types.InvocationTypeEvent,
		Payload:        payload,
	})
	return err
}

// Worker handles WorkerEvents. Its fields are functions so it can be tested
// without a database.
type Worker struct {
	Classify func(ctx context.Context, itemID uuid.UUID) error
	Preview  func(ctx context.Context, itemID uuid.UUID) error
	Sweep    func(ctx context.Context) (int, error)
}

// Handle routes one event. Returning an error makes Lambda retry it.
func (w Worker) Handle(ctx context.Context, ev WorkerEvent) error {
	switch {
	case ev.Sweep:
		_, err := w.Sweep(ctx)
		return err
	case ev.PreviewItemID != uuid.Nil:
		return w.Preview(ctx, ev.PreviewItemID)
	case ev.ItemID != uuid.Nil:
		return w.Classify(ctx, ev.ItemID)
	default:
		return errors.New("jobs: worker event has no itemId, previewItemId or sweep")
	}
}
