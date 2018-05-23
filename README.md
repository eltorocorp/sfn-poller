# sfn-poller
An API for setting microservice polling for against AWS SFN workflows.

## example
Setting up a service to poll a state machine for work.

```
import (
	"context"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/eltorocorp/sfn-poller/sfnpoller"
	"github.com/eltorocorp/sfn-poller/sfnpoller/pollable"
  somemicroservice "github.com/coolsoftwareprovider/somerepository"
)

func main() {
	env := environment.MustGetArgumentsFromEnvVars() // <- basically some method that resolves environment variables into a struct
	sfnAPI := sfn.New(getSession())
	poller := sfnpoller.
		New().
		RegisterTask(pollable.NewTask(somemicroservice.Handle, env.ActivityARN, env.WorkerName, env.Timeout, sfnAPI)).
		BeginPolling(context.Background())
	<-poller.Done()
}

func getSession() client.ConfigProvider {
	return session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
}

```
