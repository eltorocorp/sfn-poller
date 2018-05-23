//go:generate mockgen -destination=_mocks/generated/sfn/mock_sfn.go -source=../../vendor/github.com/aws/aws-sdk-go/service/sfn/sfniface/interface.go

package pollable_test

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/eltorocorp/sfn-poller/sfnpoller/cancellablecontext"
	"github.com/eltorocorp/sfn-poller/sfnpoller/pollable"
	mock_sfn "github.com/eltorocorp/sfn-poller/sfnpoller/pollable/_mocks/generated/sfn"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func Test_Started_Normally_BlocksUntilTaskHasStarted(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	sfnapi := mock_sfn.NewMockSFNAPI(controller)
	sfnapi.EXPECT().GetActivityTask(gomock.Any()).AnyTimes().Return(nil, errors.New("dummy error"))

	handler := func(interface{}, interface{}) (interface{}, error) { return nil, nil }
	task := pollable.NewTask(handler, "arn", "workername", 1*time.Millisecond, sfnapi)
	task.Start(cancellablecontext.New(context.Background()))

	<-task.Started()
}

// A task polls until its context is cancelled.
func Test_Start_Normally_PollsUntilParentContextIsDone(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	sfnapi := mock_sfn.NewMockSFNAPI(controller)
	sfnapi.EXPECT().GetActivityTask(gomock.Any()).MinTimes(1).Return(nil, errors.New("dummy error"))
	handler := func(interface{}, interface{}) (interface{}, error) { return nil, nil }
	task := pollable.NewTask(handler, "arn", "workername", 1*time.Millisecond, sfnapi)

	ctx := cancellablecontext.New(context.Background())

	task.Start(ctx)
	<-task.Started()

	// Pause for a moment to give the poller a chance to execute at least one task.
	time.Sleep(250 * time.Millisecond)

	// Cancel the context and wait for the task to report it's done.
	ctx.Cancel()
	<-task.Done()
}

// The task executes the requested handler
func Test_Start_Normally_ExecutesTheHandler(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	testHandlerInputEvent, _ := json.Marshal(struct{}{})

	sfnapi := mock_sfn.NewMockSFNAPI(controller)
	sfnapi.EXPECT().SendTaskSuccess(gomock.Any()).AnyTimes().Return(nil, nil)
	sfnapi.EXPECT().SendTaskHeartbeat(gomock.Any()).AnyTimes().Return(nil, nil)

	activityTaskOutput := &sfn.GetActivityTaskOutput{
		Input:     aws.String(string(testHandlerInputEvent)),
		TaskToken: aws.String("testToken"),
	}
	sfnapi.EXPECT().GetActivityTask(gomock.Any()).AnyTimes().Return(activityTaskOutput, nil)

	numberOfCallsToHandler := 0
	handler := func(interface{}, struct{}) (interface{}, error) {
		numberOfCallsToHandler++
		return nil, nil
	}

	task := pollable.NewTask(handler, "arn", "workername", 1*time.Millisecond, sfnapi)
	ctx := cancellablecontext.New(context.Background())
	task.Start(ctx)
	<-task.Started()

	// pause for a moment to give the poller a chance to register a call.
	time.Sleep(250 * time.Millisecond)

	// Cancel the context and wait for the task to report it's done.
	ctx.Cancel()
	<-task.Done()

	if numberOfCallsToHandler == 0 {
		t.Error("Expected at least one call to the handler, but received none.")
		t.Fail()
	}
}

// A task is done when it experiences an error reporting any of the following: heartbeat, success, or failure
func Test_Start_sfnserviceerror_UnblocksAndCancellsContext(t *testing.T) {

	heartbeatFrequency := 1 * time.Millisecond
	type handler func(interface{}, struct{}) (interface{}, error)
	type testCase struct {
		BuildTestCase func(*gomock.Controller) (*mock_sfn.MockSFNAPI, handler)
	}

	testCases := map[string]testCase{
		"GetActivityTaskReturnsAnError": testCase{
			func(c *gomock.Controller) (*mock_sfn.MockSFNAPI, handler) {
				sfnapi := mock_sfn.NewMockSFNAPI(c)
				sfnapi.EXPECT().GetActivityTask(gomock.Any()).AnyTimes().Return(nil, errors.New("testError"))
				handler := func(interface{}, struct{}) (interface{}, error) {
					return nil, nil
				}
				return sfnapi, handler
			},
		},
		"SendTaskSuccessReturnsAnError": testCase{
			func(c *gomock.Controller) (*mock_sfn.MockSFNAPI, handler) {
				sfnapi := mock_sfn.NewMockSFNAPI(c)
				testHandlerInputEvent, _ := json.Marshal(struct{}{})
				sfnapi.EXPECT().GetActivityTask(gomock.Any()).AnyTimes().Return(&sfn.GetActivityTaskOutput{
					Input:     aws.String(string(testHandlerInputEvent)),
					TaskToken: aws.String("testToken"),
				}, nil)
				sfnapi.EXPECT().SendTaskSuccess(gomock.Any()).AnyTimes().Return(nil, errors.New("testError"))
				handler := func(interface{}, struct{}) (interface{}, error) {
					return nil, nil
				}
				return sfnapi, handler
			},
		},
		"SendTaskFailureReturnsAnError": testCase{
			func(c *gomock.Controller) (*mock_sfn.MockSFNAPI, handler) {
				sfnapi := mock_sfn.NewMockSFNAPI(c)
				testHandlerInputEvent, _ := json.Marshal(struct{}{})
				sfnapi.EXPECT().GetActivityTask(gomock.Any()).AnyTimes().Return(&sfn.GetActivityTaskOutput{
					Input:     aws.String(string(testHandlerInputEvent)),
					TaskToken: aws.String("testToken"),
				}, nil)
				sfnapi.EXPECT().SendTaskFailure(gomock.Any()).AnyTimes().Return(nil, errors.New("testerror"))
				handler := func(interface{}, struct{}) (interface{}, error) {
					return nil, errors.New("error to trigger SendTaskFailure call")
				}
				return sfnapi, handler
			},
		},
		"SendTaskHeartbeatReturnsAnError": testCase{
			func(c *gomock.Controller) (*mock_sfn.MockSFNAPI, handler) {
				sfnapi := mock_sfn.NewMockSFNAPI(c)
				testHandlerInputEvent, _ := json.Marshal(struct{}{})
				sfnapi.EXPECT().GetActivityTask(gomock.Any()).AnyTimes().Return(&sfn.GetActivityTaskOutput{
					Input:     aws.String(string(testHandlerInputEvent)),
					TaskToken: aws.String("testToken"),
				}, nil)
				sfnapi.EXPECT().SendTaskSuccess(gomock.Any()).AnyTimes().Return(nil, nil)
				sfnapi.EXPECT().SendTaskHeartbeat(gomock.Any()).AnyTimes().Return(nil, errors.New("testerror"))
				handler := func(interface{}, struct{}) (interface{}, error) {
					// wait for a bit to allow a heartbeat to occur.
					time.Sleep(10 * heartbeatFrequency)
					return nil, nil
				}
				return sfnapi, handler
			},
		},
	}

	for testName, tc := range testCases {
		log.Printf("Starting test case: %v", testName)
		controller := gomock.NewController(t)
		defer controller.Finish()
		defer func() {
			t.Logf("Failed test case: %v", testName)
		}()

		sfnapi, handler := tc.BuildTestCase(controller)

		task := pollable.NewTask(handler, "arn", "workername", heartbeatFrequency, sfnapi)

		ctx := cancellablecontext.New(context.Background())
		task.Start(ctx)
		<-task.Started()

		// pause for a moment to give the poller a chance to register a call.
		time.Sleep(250 * time.Millisecond)

		<-task.Done()
		<-ctx.Done()
	}
}

func Test_Start_TaskTokenIsNil_ContinuesWithoutErrorUntilCancelled(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	sfnapi := mock_sfn.NewMockSFNAPI(controller)
	activityTaskOutput := &sfn.GetActivityTaskOutput{
		Input:     aws.String("input"),
		TaskToken: nil,
	}
	sfnapi.EXPECT().GetActivityTask(gomock.Any()).AnyTimes().Return(activityTaskOutput, nil)

	handler := func(interface{}, struct{}) (interface{}, error) {
		return nil, nil
	}

	task := pollable.NewTask(handler, "arn", "workername", 1*time.Millisecond, sfnapi)
	ctx := cancellablecontext.New(context.Background())
	task.Start(ctx)
	<-task.Started()

	// pause for a moment to give the poller a chance to register a call.
	time.Sleep(250 * time.Millisecond)

	// Cancel the context and wait for the task to report it's done.
	ctx.Cancel()
	<-task.Done()
}

func Test_NewTask2_Normally_ReturnsAReferenceToATask(t *testing.T) {
	handlerFn := func() {}
	resourceInfo := mockResourceInfo{}
	heartbeatInterval := time.Second
	var sfnAPI sfniface.SFNAPI = nil
	task := pollable.NewTask2(handlerFn, resourceInfo, heartbeatInterval, sfnAPI)
	assert.NotNil(t, task)
}

type mockResourceInfo struct{}

func (mockResourceInfo) ARN() string          { return "" }
func (mockResourceInfo) ActivityName() string { return "" }
