// Package stats does not send a daily count.
package stats

import (
	"context"
	"errors"
	"time"
)

func Run(version, what string) {}

func Send(ctx context.Context, version, what string, now time.Time) error {
	return nil
}

func Off() bool { return true }

func Host() string { return "" }

// installID makes up no id: this build keeps none, so a plugin like
// (plugins.go) is refused rather than sent under one.
func installID() (string, error) {
	return "", errors.New("this build keeps no install id")
}
