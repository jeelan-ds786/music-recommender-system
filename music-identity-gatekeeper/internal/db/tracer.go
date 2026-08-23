package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
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

	elapsed := time.Since(td.start)
	args := formatArgs(td.args)

	if data.Err != nil {
		t.log.Error("postgres %s failed in %s | args=%s | %s | err=%v", td.op, elapsed, args, td.sql, data.Err)
		return
	}

	t.log.Info("postgres %s %s in %s | args=%s | %s", td.op, data.CommandTag.String(), elapsed, args, td.sql)
}

func formatArgs(args []any) string {
	if len(args) == 0 {
		return "-"
	}

	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprintf("$%d=%v", i+1, a)
	}

	return strings.Join(parts, ", ")
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
