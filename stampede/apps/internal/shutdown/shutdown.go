package shutdown

import (
	"context"
	"os/signal"
	"syscall"
)

func Context() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
}
