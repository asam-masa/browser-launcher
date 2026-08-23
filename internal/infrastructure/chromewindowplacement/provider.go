package chromewindowplacement

import (
	"context"
	"errors"
	"time"

	placement "github.com/asam-masa/browser-launcher/internal/application/chromewindowplacement"
	tracking "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
)

const (
	defaultStabilityInterval = 250 * time.Millisecond
	defaultStabilityTimeout  = 15 * time.Second
	requiredStableSamples    = 3
)

var (
	ErrPlacementFailed       = errors.New("native Chrome window placement failed")
	ErrBoundsOutOfRange      = errors.New("window bounds exceed the Windows coordinate range")
	ErrIdentityChanged       = errors.New("Chrome window identity changed")
	ErrStabilityTimeout      = errors.New("Chrome window did not become stable")
	ErrUnsupportedPlatform   = errors.New("Chrome window placement is unsupported on this platform")
	errProviderUninitialized = errors.New("placement provider is not initialized")
)

type IdentityValidator interface {
	Validate(tracking.Window) (bool, error)
}

type Provider struct {
	validator         IdentityValidator
	system            windowSystem
	wait              func(context.Context, time.Duration) error
	stabilityInterval time.Duration
	stabilityTimeout  time.Duration
}

func NewProvider(validator IdentityValidator) Provider {
	return Provider{
		validator:         validator,
		system:            newWindowSystem(),
		wait:              waitContext,
		stabilityInterval: defaultStabilityInterval,
		stabilityTimeout:  defaultStabilityTimeout,
	}
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var _ placement.Placer = Provider{}
