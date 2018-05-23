// Package sfnpoller provides a 'generic' mechanism for to poll a StepFunction state machine for tasks.
package sfnpoller

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/eltorocorp/sfn-poller/sfnpoller/cancellablecontext"
	"github.com/eltorocorp/sfn-poller/sfnpoller/pollable/pollableiface"
)

// API is the sfnpoller's API.
type API struct {
	registeredTasks []pollableiface.PollableTask
	sfnAPI          sfniface.SFNAPI
	done            chan struct{}
}

// New returns a reference to a new sfnpoller API.
func New() *API {
	return &API{
		registeredTasks: make([]pollableiface.PollableTask, 0),
	}
}

// RegisterTask adds the specified task to the poller's internal list of tasks to execute.
func (a *API) RegisterTask(task pollableiface.PollableTask) *API {
	a.registeredTasks = append(a.registeredTasks, task)
	return a
}

// BeginPolling initiates polling on registered tasks.
// This method blocks until all all pollers have reported that they have started.
func (a *API) BeginPolling(parentCtx context.Context) *API {
	log.Println("Starting tasks...")
	ctx := cancellablecontext.New(parentCtx)
	for _, task := range a.registeredTasks {
		task.Start(ctx)
	}
	numberOfStartedTasks := 0
	for i := 0; numberOfStartedTasks < len(a.registeredTasks); i++ {
		log.Println("Waiting for task to report that it has started...")
		<-a.registeredTasks[i].Started()
		numberOfStartedTasks++
	}
	log.Println("All tasks have started.")
	return a
}

// Done returns a channel that blocks until all pollers have reported that they are done polling.
func (a *API) Done() <-chan struct{} {
	a.done = make(chan struct{})
	go func() {
		numberOfDoneTasks := 0
		for i := 0; numberOfDoneTasks < len(a.registeredTasks); i++ {
			<-a.registeredTasks[i].Done()
			numberOfDoneTasks++
		}
		a.done <- struct{}{}
	}()
	return a.done
}
