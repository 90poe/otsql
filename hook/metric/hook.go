package metric

import (
	"context"
	"time"

	"github.com/90poe/otsql"
	"github.com/prometheus/client_golang/prometheus"
)

type Hook struct {
	*Options
}

var _ otsql.Hook = (*Hook)(nil)

func (hook *Hook) Before(ctx context.Context, evt *otsql.Event) context.Context {
	return ctx
}

func (hook *Hook) After(ctx context.Context, evt *otsql.Event) {
	code := otsql.ErrToCode(evt.Err)

	hook.Latency.WithLabelValues(
		evt.Instance,
		evt.Database,
		string(evt.Method),
		code.String(),
	).Observe(float64(time.Since(evt.BeginAt).Microseconds()))
}

func New(opts ...Option) (*Hook, error) {
	o := newOptions(opts)

	err := o.Registerer.Register(o.Latency)
	if err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			return nil, err
		}
	}

	return &Hook{Options: o}, nil
}
