package log

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/90poe/otsql"
	"github.com/stretchr/testify/require"
)

type capturedEntry struct {
	level   Level
	message string
	fields  []any
}

type captureLogger struct {
	entries *[]capturedEntry
	fields  []any
}

func (l *captureLogger) WithFields(fields ...any) Logger {
	merged := make([]any, 0, len(l.fields)+len(fields))
	merged = append(merged, l.fields...)
	merged = append(merged, fields...)
	return &captureLogger{entries: l.entries, fields: merged}
}

func (l *captureLogger) log(level Level, message string) {
	*l.entries = append(*l.entries, capturedEntry{
		level:   level,
		message: message,
		fields:  append([]any(nil), l.fields...),
	})
}

func (l *captureLogger) Debug(message string, args ...any) {
	l.WithFields(args...).(*captureLogger).log(LevelDebug, message)
}

func (l *captureLogger) Info(message string, args ...any) {
	l.WithFields(args...).(*captureLogger).log(LevelInfo, message)
}

func (l *captureLogger) Error(message string, args ...any) {
	l.WithFields(args...).(*captureLogger).log(LevelError, message)
}

type captureBuilder struct {
	entries *[]capturedEntry
}

func (b *captureBuilder) Build(context.Context) Logger {
	return &captureLogger{entries: b.entries}
}

func fieldValue(fields []any, key string) (any, bool) {
	for i := 0; i+1 < len(fields); i += 2 {
		if k, ok := fields[i].(string); ok && k == key {
			return fields[i+1], true
		}
	}
	return nil, false
}

func newCaptureHook(t *testing.T, opts ...Option) (*Hook, *[]capturedEntry) {
	t.Helper()
	entries := &[]capturedEntry{}
	full := append([]Option{WithLogger(&captureBuilder{entries: entries})}, opts...)
	return New(full...), entries
}

func TestAfter_ErrorLogsAtErrorLevel(t *testing.T) {
	hook, entries := newCaptureHook(t)
	evt := &otsql.Event{
		Method:  otsql.MethodQuery,
		Query:   "SELECT 1",
		BeginAt: time.Now(),
		Err:     errors.New("boom"),
	}

	hook.After(context.Background(), evt)

	require.Len(t, *entries, 1)
	e := (*entries)[0]
	require.Equal(t, LevelError, e.level)
	require.Equal(t, "AccessLog", e.message)
	v, ok := fieldValue(e.fields, "error")
	require.True(t, ok, "missing error field")
	require.EqualError(t, v.(error), "boom")
}

func TestAfter_SlowLogsAtInfoLevelWithSlowField(t *testing.T) {
	hook, entries := newCaptureHook(t, WithSlow(time.Millisecond))
	evt := &otsql.Event{
		Method:  otsql.MethodQuery,
		Query:   "SELECT 1",
		BeginAt: time.Now().Add(-time.Second),
	}

	hook.After(context.Background(), evt)

	require.Len(t, *entries, 1)
	e := (*entries)[0]
	require.Equal(t, LevelInfo, e.level)
	v, ok := fieldValue(e.fields, "slow")
	require.True(t, ok, "missing slow field")
	require.Equal(t, true, v)
}

func TestAfter_MethodLevelOverrideUsed(t *testing.T) {
	hook, entries := newCaptureHook(t, WithMethodLevel(otsql.MethodQuery, LevelDebug))
	evt := &otsql.Event{
		Method:  otsql.MethodQuery,
		Query:   "SELECT 1",
		BeginAt: time.Now(),
	}

	hook.After(context.Background(), evt)

	require.Len(t, *entries, 1)
	require.Equal(t, LevelDebug, (*entries)[0].level)
}

func TestAfter_DefaultLevelUsedWhenMethodMissing(t *testing.T) {
	hook, entries := newCaptureHook(t)
	delete(hook.MethodLevels, otsql.MethodQuery)
	evt := &otsql.Event{
		Method:  otsql.MethodQuery,
		Query:   "SELECT 1",
		BeginAt: time.Now(),
	}

	hook.After(context.Background(), evt)

	require.Len(t, *entries, 1)
	require.Equal(t, LevelInfo, (*entries)[0].level)
}

func TestAfter_QueryAndParamsGated(t *testing.T) {
	hook, entries := newCaptureHook(t, WithQuery(true), WithArgs(true))
	evt := &otsql.Event{
		Method:  otsql.MethodQuery,
		Query:   "SELECT $1",
		Args:    []any{42},
		BeginAt: time.Now(),
	}

	hook.After(context.Background(), evt)

	require.Len(t, *entries, 1)
	fields := (*entries)[0].fields
	q, ok := fieldValue(fields, "query")
	require.True(t, ok)
	require.Equal(t, "SELECT $1", q)
	p, ok := fieldValue(fields, "params")
	require.True(t, ok)
	require.Equal(t, []any{42}, p)
}

func TestAfter_QueryOmittedWhenDisabled(t *testing.T) {
	hook, entries := newCaptureHook(t, WithQuery(false))
	evt := &otsql.Event{
		Method:  otsql.MethodQuery,
		Query:   "SELECT 1",
		BeginAt: time.Now(),
	}

	hook.After(context.Background(), evt)

	require.Len(t, *entries, 1)
	_, ok := fieldValue((*entries)[0].fields, "query")
	require.False(t, ok)
}

func TestNoopBuilder_IsDefault(t *testing.T) {
	hook := New()
	require.NotPanics(t, func() {
		hook.After(context.Background(), &otsql.Event{
			Method:  otsql.MethodQuery,
			BeginAt: time.Now(),
		})
	})
}

func TestAfter_LevelOff(t *testing.T) {
	tests := []struct {
		name       string
		opts       []Option
		evt        *otsql.Event
		wantLogged bool
		wantLevel  Level
	}{
		{
			name: "method set to off is dropped",
			opts: []Option{WithMethodLevel(otsql.MethodPing, LevelOff)},
			evt:  &otsql.Event{Method: otsql.MethodPing, BeginAt: time.Now()},
		},
		{
			name: "other methods still log",
			opts: []Option{WithMethodLevel(otsql.MethodPing, LevelOff)},
			evt: &otsql.Event{
				Method:  otsql.MethodQuery,
				Query:   "SELECT 1",
				BeginAt: time.Now(),
			},
			wantLogged: true,
			wantLevel:  LevelDebug,
		},
		{
			name: "error still logs when method is off",
			opts: []Option{WithMethodLevel(otsql.MethodPing, LevelOff)},
			evt: &otsql.Event{
				Method:  otsql.MethodPing,
				BeginAt: time.Now(),
				Err:     errors.New("boom"),
			},
			wantLogged: true,
			wantLevel:  LevelError,
		},
		{
			name: "slow still logs when method is off",
			opts: []Option{
				WithMethodLevel(otsql.MethodPing, LevelOff),
				WithSlow(time.Millisecond),
			},
			evt: &otsql.Event{
				Method:  otsql.MethodPing,
				BeginAt: time.Now().Add(-time.Second),
			},
			wantLogged: true,
			wantLevel:  LevelInfo,
		},
		{
			name: "default level off drops unmapped methods",
			opts: []Option{WithDefaultLevel(LevelOff)},
			evt: &otsql.Event{
				Method:  otsql.Method("unmapped"),
				BeginAt: time.Now(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook, entries := newCaptureHook(t, tt.opts...)

			hook.After(context.Background(), tt.evt)

			if !tt.wantLogged {
				require.Empty(t, *entries)
				return
			}

			require.Len(t, *entries, 1)
			require.Equal(t, tt.wantLevel, (*entries)[0].level)
		})
	}
}
