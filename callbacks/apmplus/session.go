package apmplus

import "context"

type sessionOptions struct {
	UserID    string
	SessionID string
}

type apmplusSessionOptionKey struct{}

func SetSession(ctx context.Context, opts ...SessionOption) context.Context {
	options := &sessionOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return context.WithValue(ctx, apmplusSessionOptionKey{}, options)
}

type SessionOption func(*sessionOptions)

func WithUserID(userID string) SessionOption {
	return func(o *sessionOptions) {
		o.UserID = userID
	}
}
func WithSessionID(sessionID string) SessionOption {
	return func(o *sessionOptions) {
		o.SessionID = sessionID
	}
}
