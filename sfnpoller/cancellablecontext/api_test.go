package cancellablecontext_test

import (
	"context"
	"testing"

	"github.com/eltorocorp/sfn-poller/sfnpoller/cancellablecontext"
	"github.com/stretchr/testify/assert"
)

func Test_New_Normally_ReturnsAPI(t *testing.T) {
	c := cancellablecontext.New(context.Background())
	assert.IsType(t, c, &cancellablecontext.API{})
}

func Test_Cancel_Normally_CancelsTheContext(t *testing.T) {
	c := cancellablecontext.New(context.Background())
	c.Cancel()
	<-c.Done()
}
