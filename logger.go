package db

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/guestin/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// Colors
const (
	Reset       = "\033[0m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Blue        = "\033[34m"
	Magenta     = "\033[35m"
	Cyan        = "\033[36m"
	White       = "\033[37m"
	BlueBold    = "\033[34;1m"
	MagentaBold = "\033[35;1m"
	RedBold     = "\033[31;1m"
	YellowBold  = "\033[33;1m"
)

func newTraceLogger(rootLogger log.ZapLog, config Config) gormLogger.Interface {
	if config.SlowThresholdMs == 0 {
		config.SlowThresholdMs = 200
	}
	if config.Colorful == nil {
		config.Colorful = new(bool)
		*config.Colorful = true
	}
	var (
		infoStr      = "%s\n"
		warnStr      = "%s\n"
		errStr       = "%s\n"
		traceStr     = "%s\n[%.3fms] [rows:%v] %s"
		traceWarnStr = "%s %s\n[%.3fms] [rows:%v] %s"
		traceErrStr  = "%s %s\n[%.3fms] [rows:%v] %s"
	)

	if *config.Colorful {
		infoStr = Green + "%s" + Reset + "\n"
		warnStr = BlueBold + "%s" + Reset + "\n"
		errStr = Magenta + "%s" + Reset + "\n"
		traceStr = Green + "%s" + Reset + "\n" + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
		traceWarnStr = Green + "%s " + Yellow + "%s" + Reset + "\n" + RedBold + "[%.3fms] " + Yellow + "[rows:%v]" + Magenta + " %s" + Reset
		traceErrStr = RedBold + "%s " + MagentaBold + "%s" + Reset + "\n" + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
	}
	return &traceLogger{
		Config: gormLogger.Config{
			SlowThreshold:             time.Millisecond * time.Duration(config.SlowThresholdMs),
			Colorful:                  *config.Colorful,
			IgnoreRecordNotFoundError: false,
			ParameterizedQueries:      false,
			LogLevel:                  gormLogger.Warn,
		},
		zapLogger:    rootLogger.With(log.WithZapOptions(zap.WithCaller(false))),
		config:       config,
		infoStr:      infoStr,
		warnStr:      warnStr,
		errStr:       errStr,
		traceStr:     traceStr,
		traceWarnStr: traceWarnStr,
		traceErrStr:  traceErrStr,
	}
}

type traceLogger struct {
	gormLogger.Config
	config                              Config
	zapLogger                           log.ZapLog
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string
}

func (l *traceLogger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info print info
func (l *traceLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Info {
		l.Printf(ctx, gormLogger.Info, l.infoStr+msg, append([]interface{}{fileWithLineNum(ctx)}, data...)...)
	}
}

// Warn print warn messages
func (l *traceLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Warn {
		l.Printf(ctx, gormLogger.Warn, l.warnStr+msg, append([]interface{}{fileWithLineNum(ctx)}, data...)...)
	}
}

// Error print error messages
func (l *traceLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Error {
		l.Printf(ctx, gormLogger.Error, l.errStr+msg, append([]interface{}{fileWithLineNum(ctx)}, data...)...)
	}
}

func (l *traceLogger) rowStr(row int64) string {
	if row == -1 {
		return "-"
	}
	return fmt.Sprintf("%d", row)
}

// Trace print sql message
//
//nolint:cyclop
func (l *traceLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormLogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= gormLogger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		l.Printf(ctx, gormLogger.Warn, l.traceErrStr, fileWithLineNum(ctx), err, float64(elapsed.Nanoseconds())/1e6, l.rowStr(rows), sql)
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= gormLogger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		l.Printf(ctx, gormLogger.Warn, l.traceWarnStr, fileWithLineNum(ctx), slowLog, float64(elapsed.Nanoseconds())/1e6, l.rowStr(rows), sql)
	case l.LogLevel == gormLogger.Info:
		sql, rows := fc()
		l.Printf(ctx, gormLogger.Info, l.traceStr, fileWithLineNum(ctx), float64(elapsed.Nanoseconds())/1e6, l.rowStr(rows), sql)
	}
}

func (l *traceLogger) ParamsFilter(ctx context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if l.Config.ParameterizedQueries {
		return sql, nil
	}
	return sql, params
}

func (l *traceLogger) Printf(ctx context.Context, lv gormLogger.LogLevel, s string, i ...interface{}) {
	traceId := _traceId(ctx)
	traceIdPrefix := ""
	if traceId != "" {
		traceIdPrefix = fmt.Sprintf("[%s] ", traceId)
	}
	switch lv {
	case gormLogger.Error:
		lines := strings.Split(fmt.Sprintf(s, i...), "\n")
		for _, line := range lines {
			l.zapLogger.
				Error(fmt.Sprintf("%s%s", traceIdPrefix, line))
		}
	case gormLogger.Warn:
		lines := strings.Split(fmt.Sprintf(s, i...), "\n")
		for _, line := range lines {
			l.zapLogger.
				Warn(fmt.Sprintf("%s%s", traceIdPrefix, line))
		}
	case gormLogger.Info:
		lines := strings.Split(fmt.Sprintf(s, i...), "\n")
		for _, line := range lines {
			l.zapLogger.
				Info(fmt.Sprintf("%s%s", traceIdPrefix, line))
		}
	default:
		return
	}
}

func _traceId(ctx context.Context) string {
	if ctx != nil {
		i := ctx.Value(CtxTraceIdKey)
		if i != nil {
			return i.(string)
		}
	}
	return ""
}

func _traceSkip(ctx context.Context) int {
	i := ctx.Value(CtxTraceSkipKey)
	if i != nil {
		return i.(int)
	}
	return 0
}

// // FileWithLineNum return the file name and line number of the current file
func fileWithLineNum(ctx context.Context) string {
	pcs := [13]uintptr{}
	// the third caller usually from gorm internal
	len := runtime.Callers(5+_traceSkip(ctx), pcs[:])
	frames := runtime.CallersFrames(pcs[:len])
	for i := 0; i < len; i++ {
		// second return value is "more", not "ok"
		frame, _ := frames.Next()
		if (!strings.HasPrefix(frame.File, "gormSourceDir") ||
			strings.HasSuffix(frame.File, "_test.go")) && !strings.HasSuffix(frame.File, ".gen.go") {
			return string(strconv.AppendInt(append([]byte(frame.File), ':'), int64(frame.Line), 10))
		}
	}

	return ""
}
