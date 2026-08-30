package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
)

type queryTracer struct {
	log *logger.Logger
}

// NewQueryTracer logs every query pgx sends to Postgres, tagged READ/WRITE,
// at the logger's Info level. Wire it into pgxpool.Config.ConnConfig.Tracer.
func NewQueryTracer(log *logger.Logger) pgx.QueryTracer {
	return &queryTracer{log: log}
}

type traceCtxKey struct{}

type traceData struct {
	sql   string
	args  []any
	op    string
	start time.Time
}

func (t *queryTracer) TraceQueryStart(
	ctx context.Context,
	_ *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {

	return context.WithValue(ctx, traceCtxKey{}, &traceData{
		sql:   data.SQL,
		args:  data.Args,
		op:    operationOf(data.SQL),
		start: time.Now(),
	})
}

func (t *queryTracer) TraceQueryEnd(
	ctx context.Context,
	_ *pgx.Conn,
	data pgx.TraceQueryEndData,
) {

	td, ok := ctx.Value(traceCtxKey{}).(*traceData)
	if !ok {
		return
	}

	rid, _ := reqid.FromContext(ctx)
	elapsed := time.Since(td.start)
	args := formatArgs(td.args)

	if data.Err != nil {
		t.log.Error(rid, "postgres %s failed in %s | args=%s | %s | err=%v", td.op, elapsed, args, td.sql, data.Err)
		return
	}

	t.log.Info(rid, "postgres %s %s in %s | args=%s | %s", td.op, data.CommandTag.String(), elapsed, args, td.sql)
}

// maxArgLen bounds how much of a single argument's formatted value is
// logged, so a large JSON payload (or any other big []byte arg) doesn't
// flood the log.
const maxArgLen = 500

func formatArgs(args []any) string {
	if len(args) == 0 {
		return "-"
	}

	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprintf("$%d=%s", i+1, formatArg(a))
	}

	return strings.Join(parts, ", ")
}

// formatArg renders a single bound query argument for logging. []byte
// args (e.g. a JSONB payload) are rendered as their string content via %s
// instead of %v, which would otherwise print an unreadable decimal byte
// dump like "[123 34 109 ...]".
func formatArg(a any) string {
	var s string
	if b, ok := a.([]byte); ok {
		s = string(b)
	} else {
		s = fmt.Sprintf("%v", a)
	}

	if len(s) > maxArgLen {
		return s[:maxArgLen] + "...(truncated)"
	}

	return s
}

func operationOf(sql string) string {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return "UNKNOWN"
	}

	switch strings.ToUpper(fields[0]) {
	case "SELECT":
		return "READ"
	case "INSERT", "UPDATE", "DELETE":
		return "WRITE"
	default:
		return strings.ToUpper(fields[0])
	}
}
