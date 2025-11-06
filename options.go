package db

type (
	_ormCxt struct {
		dbSelect   string
		traceId    string
		callerSkip int
	}
	Option interface {
		apply(ctx *_ormCxt)
	}
	optionFunc func(ctx *_ormCxt)
)

func (f optionFunc) apply(ctx *_ormCxt) {
	f(ctx)
}

func UseDb(name string) Option {
	return optionFunc(func(ctx *_ormCxt) {
		ctx.dbSelect = name
	})
}

func TraceId(traceId string) Option {
	return optionFunc(func(ctx *_ormCxt) {
		ctx.traceId = traceId
	})
}

func CallerSkip(skip int) Option {
	return optionFunc(func(ctx *_ormCxt) {
		ctx.callerSkip = skip
	})
}
