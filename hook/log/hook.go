package log

import (
	"context"
	"time"

	"github.com/90poe/otsql"
)

// Logger is the minimal logging interface required by the hook. It is
// intentionally implementation-agnostic: args are flat key/value tuples
// (k1, v1, k2, v2, ...).
type Logger interface {
	Debug(message string, args ...any)
	Info(message string, args ...any)
	Error(message string, args ...any)
	WithFields(fields ...any) Logger
}

// ContextLoggerBuilder produces a Logger bound to a request context.
type ContextLoggerBuilder interface {
	Build(ctx context.Context) Logger
}

// Level is the log level used to dispatch a log entry to one of the
// Logger methods.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelError

	// LevelOff drops the entry instead of logging it. It is only reachable
	// through DefaultLevel or a MethodLevels override, so a failed or slow
	// event is still logged: those set their level before the override is
	// consulted. Use it to silence routine methods such as ping while
	// keeping the errors and the slow-query warnings they can produce.
	LevelOff
)

// Hook is an otsql.Hook that emits an access log entry after every SQL event.
type Hook struct {
	*Options
}

var _ otsql.Hook = (*Hook)(nil)

func (hook *Hook) Before(ctx context.Context, evt *otsql.Event) context.Context {
	return ctx
}

func (hook *Hook) After(ctx context.Context, evt *otsql.Event) {
	latency := time.Since(evt.BeginAt)

	var level Level
	slow := false
	switch {
	case evt.Err != nil:
		level = LevelError
	case latency > hook.Slow:
		level = LevelInfo
		slow = true
	default:
		l, ok := hook.MethodLevels[evt.Method]
		if !ok {
			l = hook.DefaultLevel
		}
		level = l
	}

	if level == LevelOff {
		return
	}

	fields := make([]any, 0, 20)
	fields = append(fields, "kind", "sql")
	if evt.Instance != "" {
		fields = append(fields, "server", evt.Instance)
	}
	if evt.Conn != "" {
		fields = append(fields, "conn", evt.Conn)
	}
	if evt.Database != "" {
		fields = append(fields, "database", evt.Database)
	}
	if evt.Method != "" {
		fields = append(fields, "method", string(evt.Method))
	}
	if slow {
		fields = append(fields, "slow", true)
	}
	fields = append(fields,
		"code", otsql.ErrToCode(evt.Err).String(),
		"latency", latency,
	)
	if hook.Query && evt.Query != "" {
		fields = append(fields, "query", evt.Query)
		if hook.Args && evt.Args != nil {
			fields = append(fields, "params", evt.Args)
		}
	}
	// append error at the end
	if evt.Err != nil {
		fields = append(fields, "error", evt.Err)
	}

	logger := hook.Builder.Build(ctx).WithFields(fields...)
	switch level {
	case LevelDebug:
		logger.Debug("AccessLog")
	case LevelError:
		logger.Error("AccessLog")
	default:
		logger.Info("AccessLog")
	}
}

func New(opts ...Option) *Hook {
	return &Hook{Options: newOptions(opts)}
}

// Option configures a Hook.
type Option func(*Options)

// Options is the resolved configuration of a Hook.
type Options struct {
	Builder      ContextLoggerBuilder
	DefaultLevel Level
	MethodLevels map[otsql.Method]Level

	Slow time.Duration

	Query bool
	Args  bool
}

func newOptions(opts []Option) *Options {
	o := &Options{
		Builder:      noopBuilder{},
		DefaultLevel: LevelInfo,
		Slow:         time.Second * 3,

		MethodLevels: map[otsql.Method]Level{
			otsql.MethodPing:     LevelDebug,
			otsql.MethodQuery:    LevelDebug,
			otsql.MethodPrepare:  LevelDebug,
			otsql.MethodBegin:    LevelDebug,
			otsql.MethodCommit:   LevelDebug,
			otsql.MethodRollback: LevelDebug,

			otsql.MethodLastInsertId: LevelDebug,
			otsql.MethodRowsAffected: LevelDebug,
			otsql.MethodRowsClose:    LevelDebug,
			otsql.MethodRowsNext:     LevelDebug,

			otsql.MethodExec:         LevelInfo,
			otsql.MethodCreateConn:   LevelInfo,
			otsql.MethodCloseConn:    LevelInfo,
			otsql.MethodResetSession: LevelDebug,
		},

		Query: true,
		Args:  false,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithLogger installs a ContextLoggerBuilder together with the default level
// to use when a method has no explicit override. Both arguments are required:
// it is impossible to install a logger without choosing a default level.
func WithLogger(builder ContextLoggerBuilder) Option {
	return func(o *Options) {
		o.Builder = builder
	}
}

func WithDefaultLevel(level Level) Option {
	return func(o *Options) {
		o.DefaultLevel = level
	}
}

func WithSlow(d time.Duration) Option {
	return func(o *Options) {
		o.Slow = d
	}
}

func WithMethodLevel(method otsql.Method, level Level) Option {
	return func(o *Options) {
		o.MethodLevels[method] = level
	}
}

func WithQuery(b bool) Option {
	return func(o *Options) {
		o.Query = b
	}
}

func WithArgs(b bool) Option {
	return func(o *Options) {
		o.Args = b
	}
}

// noopBuilder is the default builder when WithLogger is not called. It
// produces a Logger that discards every entry, keeping the hook silent
// rather than forcing a dependency on any concrete log implementation.
type noopBuilder struct{}

func (noopBuilder) Build(context.Context) Logger { return noopLogger{} } //nolint: ireturn

type noopLogger struct{}

func (noopLogger) Debug(string, ...any)     {}
func (noopLogger) Info(string, ...any)      {}
func (noopLogger) Error(string, ...any)     {}
func (noopLogger) WithFields(...any) Logger { return noopLogger{} } //nolint: ireturn
