package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/flightctl/flightctl/api/v1alpha1"
	"github.com/flightctl/flightctl/internal/api/client"
)

func NewEnrollment(
	client *client.ClientWithResponses,
) *Enrollment {
	return &Enrollment{
		client: client,
	}
}

type Enrollment struct {
	client                 *client.ClientWithResponses
	rpcMetricsCallbackFunc func(operation string, durationSeconds float64, err error)
}

func (e *Enrollment) SetRPCMetricsCallback(cb func(operation string, durationSeconds float64, err error)) {
	e.rpcMetricsCallbackFunc = cb
}

func (e *Enrollment) CreateEnrollmentRequest(ctx context.Context, req v1alpha1.EnrollmentRequest, cb ...client.RequestEditorFn) (*v1alpha1.EnrollmentRequest, error) {
	start := time.Now()
	resp, err := e.client.CreateEnrollmentRequestWithResponse(ctx, req, cb...)
	if err != nil {
		return nil, err
	}
	if e.rpcMetricsCallbackFunc != nil {
		e.rpcMetricsCallbackFunc("create_enrollmentrequest_duration", time.Since(start).Seconds(), err)
	}

	if resp.StatusCode() != http.StatusCreated {
		return nil, fmt.Errorf("create enrollmentrequest failed: %s", resp.Status())
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("create enrollmentrequest failed: %s", ErrEmptyResponse)
	}

	return resp.JSON201, nil
}

func (e *Enrollment) GetEnrollmentRequest(ctx context.Context, id string, cb ...client.RequestEditorFn) (*v1alpha1.EnrollmentRequest, error) {
	start := time.Now()
	resp, err := e.client.ReadEnrollmentRequestWithResponse(ctx, id, cb...)
	if err != nil {
		return nil, err
	}
	if e.rpcMetricsCallbackFunc != nil {
		e.rpcMetricsCallbackFunc("get_enrollmentrequest_duration", time.Since(start).Seconds(), err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("get enrollmentrequest failed: %s", resp.Status())
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("get enrollmentrequest failed: %s", ErrEmptyResponse)
	}

	return resp.JSON200, nil
}
