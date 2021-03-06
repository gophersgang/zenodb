package core

import (
	"context"
	"github.com/getlantern/bytemap"
	"time"
)

func Flatten(source RowSource) FlatRowSource {
	return &flatten{rowTransform{source}}
}

type flatten struct {
	rowTransform
}

func (f *flatten) Iterate(ctx context.Context, onRow OnFlatRow) error {
	fields := f.GetFields()
	numFields := len(fields)
	resolution := f.GetResolution()

	return f.source.Iterate(ctx, func(key bytemap.ByteMap, vals Vals) (bool, error) {
		var until time.Time
		var asOf time.Time
		// Figure out total time range
		for i, field := range fields {
			val := vals[i]
			e := field.Expr
			width := e.EncodedWidth()
			if val.NumPeriods(width) == 0 {
				continue
			}
			newUntil := val.Until()
			newAsOf := val.AsOf(width, resolution)
			if newUntil.After(until) {
				until = newUntil
			}
			if asOf.IsZero() || newAsOf.Before(asOf) {
				asOf = newAsOf
			}
		}

		// Iterate
		ts := asOf
		for ; !ts.After(until); ts = ts.Add(resolution) {
			tsNanos := ts.UnixNano()
			row := &FlatRow{
				TS:     tsNanos,
				Key:    key,
				Values: make([]float64, numFields),
				fields: fields,
			}
			anyNonConstantValueFound := false
			for i, field := range fields {
				val, found := vals[i].ValueAtTime(ts, field.Expr, resolution)
				if found && !field.Expr.IsConstant() {
					anyNonConstantValueFound = true
				}
				row.Values[i] = val
			}
			if anyNonConstantValueFound {
				more, err := onRow(row)
				if !more || err != nil {
					return more, err
				}
			}
		}

		return proceed()
	})
}

func (f *flatten) String() string {
	return "flatten"
}
