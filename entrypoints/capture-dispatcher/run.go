package entrypoint

import (
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/lambdawalker/go.apexfission.aws.auth/awscapture"
	"github.com/lambdawalker/go.apexfission.aws.auth/capture"
	"log"
	"log/slog"
	"time"
)

func Run() {
	_, store, transport, e := awscapture.New(context.Background())
	if e != nil {
		log.Fatal("capture initialization failed")
	}
	lambda.Start(func(ctx context.Context) error {
		sent := 0
		err := store.Pending(ctx, time.Now().Unix(), func(job capture.Job) error {
			err := transport.Send(ctx, job)
			slog.Info("capture.dispatch", "capture_id", job.ID, "work", job.Work, "failed", err != nil)
			if err == nil {
				sent++
			}
			return err
		})
		slog.Info("capture.sweep", "sent", sent, "failed", err != nil)
		return err
	})
}
