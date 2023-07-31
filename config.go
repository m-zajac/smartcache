package smartcache

import (
	"errors"
	"time"
)

type config struct {
	primaryTTL             time.Duration
	secondaryTTL           time.Duration
	backgroundFetchTimeout time.Duration
	backgroundErrorHandler BackgroundErrorHandler
	errorTTLFunc           ErrorTTLFunc
}

// Options allows to configure cache settings.
type Option func(*config) error

// WithTTL sets the primary and secondary TTLs.
func WithTTL(primaryTTL, secondaryTTL time.Duration) Option {
	return func(c *config) error {
		if primaryTTL == 0 {
			return errors.New("primaryTTL has to be > 0")
		}
		if secondaryTTL <= primaryTTL {
			return errors.New("secondaryTTL has to be > primaryTTL")
		}

		c.primaryTTL = primaryTTL
		c.secondaryTTL = secondaryTTL

		return nil
	}
}

// WithErrorTTLFunc allows caching errors. Cache expiry time is determined by the provided function.
// If function returns 0 for an error, it won't be cached.
func WithErrorTTLFunc(f ErrorTTLFunc) Option {
	return func(c *config) error {
		if f != nil {
			c.errorTTLFunc = f
		}

		return nil
	}
}

// WithBackgroundFetchTimeout allows setting a timeout for the background fetch function.
func WithBackgroundFetchTimeout(timeout time.Duration) Option {
	return func(c *config) error {
		if timeout <= 0 {
			return errors.New("timeout has to be > 0")
		}

		c.backgroundFetchTimeout = timeout

		return nil
	}
}

// WithBackgroundFetchErrorHandler allows adding a handler for background fetch errors.
func WithBackgroundFetchErrorHandler(backgroundErrorHandler BackgroundErrorHandler) Option {
	return func(c *config) error {
		if backgroundErrorHandler != nil {
			c.backgroundErrorHandler = backgroundErrorHandler
		}

		return nil
	}
}
