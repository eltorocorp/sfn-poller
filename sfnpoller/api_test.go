//go:generate mockwrap -destination="_mocks/generated/pollabletask/mock_pollabletask.go" -package=mock_pollabletask github.com/eltorocorp/sfn-poller/sfnpoller/pollable/pollableiface PollableTask
package sfnpoller_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/eltorocorp/sfn-poller/sfnpoller"
	"github.com/eltorocorp/sfn-poller/sfnpoller/_mocks/generated/pollabletask"
	"github.com/eltorocorp/sfn-poller/sfnpoller/cancellablecontext"

	"github.com/stretchr/testify/assert"
)

func Test_New_Normally_ReturnsANewAPI(t *testing.T) {
	poller := sfnpoller.New()
	assert.NotNil(t, poller)
}

func Test_RegisterTask_Normally_ReturnsAPI(t *testing.T) {
	poller := sfnpoller.New()
	api := poller.RegisterTask(nil)
	assert.NotNil(t, api)
}

func Test_BeginPolling_Normally_InitiatesPollingForAllRegisteredTasks(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	pollableTaskOne := mock_pollabletask.NewMockPollableTask(controller)
	pollableTaskTwo := mock_pollabletask.NewMockPollableTask(controller)

	pollableTaskOne.EXPECT().Start(gomock.Any())
	pollableTaskTwo.EXPECT().Start(gomock.Any())

	returnStarted := func() <-chan struct{} {
		started := make(chan struct{}, 1)
		started <- struct{}{}
		return started
	}
	pollableTaskOne.EXPECT().Started().DoAndReturn(returnStarted)
	pollableTaskTwo.EXPECT().Started().DoAndReturn(returnStarted)

	sfnpoller.
		New().
		RegisterTask(pollableTaskOne).
		RegisterTask(pollableTaskTwo).
		BeginPolling(context.Background())
}

func Test_BeginPolling_Normally_BlocksUntilAllPollersHaveStarted(t *testing.T) {
	// If BeginPolling returns before the pollers have started, the test function will exit before
	// the pollableTask mock has registered a completed call with the mock controller.
	// The test is repeating a large number of times to ensure the result is consistent; not an artifact of timing.
	for i := 0; i < 1000; i++ {
		controller := gomock.NewController(t)
		controller.Finish()

		pollableTask := mock_pollabletask.NewMockPollableTask(controller)
		pollableTask.EXPECT().Start(gomock.Any()).Times(1)
		pollableTask.EXPECT().Started().DoAndReturn(func() <-chan struct{} {
			started := make(chan struct{})
			go func() {
				started <- struct{}{}
			}()
			return started
		})

		sfnpoller.
			New().
			RegisterTask(pollableTask).
			BeginPolling(context.Background())
	}
}

func Test_Done_Normally_BlocksUntilAllPollersStop(t *testing.T) {
	// If the poller.Done() method is not blocking, this test function will exit before
	// the mock controller has registered a call to the Done method, and the test will fail.

	controller := gomock.NewController(t)
	defer controller.Finish()

	pollableTask := mock_pollabletask.NewMockPollableTask(controller)
	pollableTask.EXPECT().Start(gomock.Any())

	pollableTask.EXPECT().Started().DoAndReturn(func() <-chan struct{} {
		started := make(chan struct{})
		go func() {
			started <- struct{}{}
		}()
		return started
	})

	pollableTask.EXPECT().Done().DoAndReturn(func() <-chan struct{} {
		done := make(chan struct{})
		go func() {
			done <- struct{}{}
		}()
		return done
	})

	poller := sfnpoller.
		New().
		RegisterTask(pollableTask).
		BeginPolling(context.Background())

	<-poller.Done()
}

type sameType struct {
	x interface{}
}

func (s sameType) Matches(x interface{}) bool {
	return reflect.TypeOf(s.x) == reflect.TypeOf(x)
}

func (s sameType) String() string {
	return reflect.TypeOf(s.x).String()
}

func SameType(x interface{}) gomock.Matcher {
	return sameType{x}
}

// The poller's context is shared with each pollable as a cancellable context.
func Test_BeginPolling_Normally_SharesParentContextWithChildrenAsCancellable(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	pollableTask := mock_pollabletask.NewMockPollableTask(controller)
	pollableTask.EXPECT().Start(SameType(cancellablecontext.New(context.Background())))
	pollableTask.EXPECT().Started().DoAndReturn(func() <-chan struct{} {
		started := make(chan struct{}, 1)
		started <- struct{}{}
		return started
	})
	sfnpoller.New().RegisterTask(pollableTask).BeginPolling(context.Background())
}
