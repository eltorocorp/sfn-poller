// Package pollable contains mechanisms for setting up and managing a task that can poll SFN for work.
package pollable

import (
	"log"
	"reflect"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/eltorocorp/sfn-poller/sfnpoller/cancellablecontext/cancellablecontextiface"
	"github.com/eltorocorp/sfn-poller/sfnpoller/internal/util"
	"github.com/eltorocorp/sfn-poller/sfnpoller/pollable/pollableiface"
)

// Task is an action that supports polling.
type Task struct {
	handlerFn         interface{}
	activityArn       string
	workerName        string
	heartbeatInterval time.Duration
	sfnAPI            sfniface.SFNAPI
	started           chan struct{}
	done              chan struct{}
}

// NewTask returns a reference to a new pollable task.
func NewTask(handlerFn interface{}, activityArn, workerName string, heartbeatInterval time.Duration, sfnAPI sfniface.SFNAPI) *Task {
	return &Task{
		handlerFn:         handlerFn,
		activityArn:       activityArn,
		workerName:        workerName,
		heartbeatInterval: heartbeatInterval,
		sfnAPI:            sfnAPI,
	}
}

// ResourceInfo is the interface for any resource that knows its ARN and ActivityName.
type ResourceInfo interface {
	ARN() string
	ActivityName() string
}

// NewTask2 returns a reference to a new pollable task using ResourceInfo.
func NewTask2(handlerFn interface{}, resourceInfo ResourceInfo, heartbeatInterval time.Duration, sfnAPI sfniface.SFNAPI) *Task {
	return NewTask(handlerFn, resourceInfo.ARN(), resourceInfo.ActivityName(), heartbeatInterval, sfnAPI)
}

// Start initializes polling for the task.
func (task *Task) Start(ctx cancellablecontextiface.Context) {
	task.started = make(chan struct{})
	task.done = make(chan struct{})
	go func() {
		defer close(task.started)
		defer close(task.done)
		task.started <- struct{}{}
		for {
			var ctxDone bool
			select {
			case <-ctx.Done():
				ctxDone = true
			default:
				ctxDone = false
			}

			if ctxDone == true {
				task.done <- struct{}{}
				log.Println("Task execution done.")
				break
			}

			getActivityTaskOutput, err := task.sfnAPI.GetActivityTask(&sfn.GetActivityTaskInput{
				ActivityArn: aws.String(task.activityArn),
				WorkerName:  aws.String(task.workerName),
			})
			if err != nil {
				log.Println(err)
				ctx.Cancel()
				break
			}
			if getActivityTaskOutput.TaskToken == nil {
				continue
			}

			log.Printf("%v starting work on task token: %v...", task.workerName, *getActivityTaskOutput.TaskToken)

			handler := reflect.ValueOf(task.handlerFn)
			handlerType := reflect.TypeOf(task.handlerFn)
			eventType := handlerType.In(1)
			event := reflect.New(eventType)
			ctxValue := reflect.ValueOf(ctx)
			util.MustUnmarshal(getActivityTaskOutput.Input, event.Interface())
			args := []reflect.Value{
				ctxValue,
				event.Elem(),
			}
			out, err := task.keepAlive(handler.Call, args, getActivityTaskOutput.TaskToken, task.heartbeatInterval)
			if err != nil {
				log.Println("An error occured while reporting a heartbeat to SFN!")
				log.Println(err)
				ctx.Cancel()
				break
			}

			result := out[0].Interface()
			var callErr error
			if !out[1].IsNil() {
				callErr = out[1].Interface().(error)
			}
			if callErr != nil {
				log.Printf("%v sending failure notification to SFN... %s", task.workerName, *getActivityTaskOutput.TaskToken)
				_, err := task.sfnAPI.SendTaskFailure(&sfn.SendTaskFailureInput{
					Cause:     aws.String(callErr.Error()),
					Error:     aws.String(callErr.Error()),
					TaskToken: getActivityTaskOutput.TaskToken,
				})
				if err != nil {
					log.Println("An error occured while reporting failure to SFN!")
					log.Println(err)
					ctx.Cancel()
					break
				}
			} else {
				log.Printf("%v sending success notification to SFN... %s", task.workerName, *getActivityTaskOutput.TaskToken)
				taskOutputJSON := util.MustMarshal(result)
				_, err := task.sfnAPI.SendTaskSuccess(&sfn.SendTaskSuccessInput{
					Output:    taskOutputJSON,
					TaskToken: getActivityTaskOutput.TaskToken,
				})
				if err != nil {
					log.Println("An error occured while reporting success to SFN!")
					log.Println(err)
					ctx.Cancel()
					break
				}
			}
		}
	}()
}

// Done returns a channel that blocks until the task is done polling.
func (task *Task) Done() <-chan struct{} {
	return task.done
}

// Started returns a channel that blocks until the task has started polling.
func (task *Task) Started() <-chan struct{} {
	return task.started
}

// keepAlive calls the handler function then periodicially sends heartbeat notifications to SFN until the handler function returns.
// This method blocks until the handler returns.
func (task *Task) keepAlive(handler func([]reflect.Value) []reflect.Value, args []reflect.Value, taskToken *string, heartbeatInterval time.Duration) (result []reflect.Value, err error) {
	resultSource := make(chan []reflect.Value, 1)
	go func() {
		resultSource <- handler(args)
		close(resultSource)
	}()
	for {
		select {
		case result = <-resultSource:
			return
		case <-time.After(heartbeatInterval):
			log.Println("Heartbeat")
			_, err = task.sfnAPI.SendTaskHeartbeat(&sfn.SendTaskHeartbeatInput{
				TaskToken: taskToken,
			})
			if err != nil {
				return
			}
		}
	}
}

var _ pollableiface.PollableTask = (*Task)(nil)
