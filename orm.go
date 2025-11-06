package db

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/guestin/kboot"
	"github.com/ooopSnake/assert.go"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var _ormDB *gorm.DB

var _ormMaps = new(sync.Map)

var _migrator MigrateFunc

type MigrateFunc func() error

func SetupMigrateBuilder(migrator MigrateFunc) {
	_migrator = migrator
}

func getDB(name string) *gorm.DB {
	if name == "" || strings.ToLower(name) == cfgKeyDefault {
		if _ormDB == nil {
			assert.Must(true, "no default db configured").Panic()
		}
		return _ormDB
	}
	ret, ok := _ormMaps.Load(strings.ToLower(name))
	assert.Must(ok, fmt.Sprintf("no such db '%s' configured", name[0])).Panic()
	return ret.(*gorm.DB)
}

// ORM get the orm instance of special name , empty name will get the default
func ORM(o ...Option) *gorm.DB {
	ctx := &_ormCxt{
		dbSelect:   "",
		traceId:    "",
		callerSkip: 0,
	}
	for _, opt := range o {
		opt.apply(ctx)
	}
	ins := getDB(ctx.dbSelect)
	insCtx := ins.Statement.Context
	if ctx.traceId != "" {
		insCtx = context.WithValue(insCtx, CtxTraceIdKey, ctx.traceId)
	}
	if ctx.callerSkip > 0 {
		insCtx = context.WithValue(insCtx, CtxTraceSkipKey, ctx.callerSkip)
	}
	return ins.WithContext(insCtx)
}

func newORM(ctx context.Context, config Config, location *time.Location) (*gorm.DB, error) {
	var dbDialer func(dsn string) gorm.Dialector
	dbConfig := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		NowFunc: func() time.Time {
			return time.Now().In(location)
		},
		Logger: newTraceLogger(kboot.GetTaggedZapLogger(ModuleName), config),
	}
	switch config.Type {
	case DsTypeSqlLite:
		dbDialer = sqlite.Open
	default:
		dbDialer = postgres.Open
	}
	orm, err := gorm.Open(dbDialer(config.DSN), dbConfig)
	if err != nil {
		return nil, err
	}
	if config.Debug {
		orm = orm.Debug()
	}
	// assign context
	orm = orm.WithContext(ctx)
	return orm, nil
}
