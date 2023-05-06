package smartcache

import "time"

type config struct {
	primaryTTL             time.Duration
	secondaryTTL           time.Duration
	backgroundFetchTimeout time.Duration
	backgroundErrorHandler BackgroundErrorHandler
	errorTTLFunc           ErrorTTLFunc
}

type Option func(*config)

func WithTTL(primaryTTL, secondaryTTL time.Duration) Option {
	return func(c *config) {
		c.primaryTTL = primaryTTL
		c.secondaryTTL = secondaryTTL
	}
}

func WithErrorTTLFunc(f ErrorTTLFunc) Option {
	return func(c *config) {
		if f != nil {
			c.errorTTLFunc = f
		}
	}
}

func WithBackgroundFetchSettings(timeout time.Duration, backgroundErrorHandler BackgroundErrorHandler) Option {
	return func(c *config) {
		c.backgroundFetchTimeout = timeout
		if backgroundErrorHandler != nil {
			c.backgroundErrorHandler = backgroundErrorHandler
		}
	}
}
