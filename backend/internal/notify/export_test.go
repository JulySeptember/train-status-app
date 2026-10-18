package notify

import (
	"time"

	"train-status-app/backend/internal/notify/webpush"
)

// テストで、送り手と時計を差し替える

func SetServiceSender(s *Service, sender Sender) {
	s.sender = func(*webpush.Keys) Sender { return sender }
}

func SetNotifierSender(n *Notifier, sender Sender) {
	n.sender = func(*webpush.Keys) Sender { return sender }
}

func SetNotifierClock(n *Notifier, now func() time.Time) {
	n.now = now
}
