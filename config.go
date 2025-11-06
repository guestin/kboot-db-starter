package db

const (
	ModuleName      = "db"
	CtxTraceIdKey   = "kboot-db-trace-id"
	CtxTraceSkipKey = "kboot-db-trace-skip"

	cfgKeyDefault           = "default"
	cfgKeyDbType            = "type"
	cfgKeyDbDsn             = "dsn"
	cfgKeyDbDebug           = "debug"
	cfgKeyDbTimezone        = "timezone"
	cfgKeyDbSlowThresholdMs = "slowThresholdMs"

	DsTypePg      = "postgres"
	DsTypeSqlLite = "sqlite"
)

type Config struct {
	name            string
	Type            string `toml:"type" validate:"required,oneof=postgres sqlite" mapstructure:"type"`
	DSN             string `toml:"dsn" validate:"required" mapstructure:"dsn"`
	Debug           bool   `toml:"debug" mapstructure:"debug"`
	SlowThresholdMs int64  `toml:"slowThresholdMs" validate:"gte=0" mapstructure:"slowThresholdMs"`
	Colorful        *bool  `toml:"colorful" mapstructure:"colorful"`
}
