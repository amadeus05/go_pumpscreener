package lognotifier

import (
	"context"
	"log"

	"pumpscreener/src/domain"
	"pumpscreener/src/infrastructure/telegram"
)

type Notifier struct{}

func New() *Notifier {
	return &Notifier{}
}

func (n *Notifier) NotifySignal(ctx context.Context, signal domain.Signal) error {
	log.Printf("PUMP SIGNAL\n%s", telegram.FormatSignal(signal))
	return nil
}
