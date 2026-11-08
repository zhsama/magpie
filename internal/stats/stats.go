// Package stats does not send a daily count.
package stats

import (
	"context"
	"time"
)

func Run(version, what string) {}

func Send(ctx context.Context, version, what string, now time.Time) error {
	return nil
}

func Off() bool { return true }

func Host() string { return "" }
