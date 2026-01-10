package routers

import "context"

type failureRecordOptions struct {
	isSingleDeployment bool
}

type failureRecordWithOptions interface {
	RecordFailureWithOptions(ctx context.Context, deploymentID string, err error, opts failureRecordOptions) error
}
