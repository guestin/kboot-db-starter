package db

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/guestin/kboot"
	"github.com/guestin/mob/merrors"
	"go.uber.org/zap"
	gormLogger "gorm.io/gorm/logger"
)

var _sourceDir string

func init() {
	_, file, _, _ := runtime.Caller(0)
	// compatible solution to get gorm source directory with various operating systems
	_sourceDir = sourceDir(file)
}

func sourceDir(file string) string {
	s := filepath.Dir(file)
	return filepath.ToSlash(s) + "/"
}

func init() {
	kboot.RegisterUnit(ModuleName, _init)
}

func _init(unit kboot.Unit) (kboot.ExecFunc, error) {
	cfgList, err := bindConfig()
	timezone := kboot.GetContext().GetTimezone()
	if err != nil {
		return nil, err
	}
	if len(cfgList) == 0 {
		return nil, merrors.Errorf("no valid db Config found")
	}
	gormLogger.Default = newTraceLogger(kboot.GetTaggedZapLogger(ModuleName), *cfgList[cfgKeyDefault])
	for _, cfg := range cfgList {
		ds := cfg.name
		orm, err := newORM(unit.GetContext(), *cfg, timezone)
		if err != nil {
			return nil, merrors.Errorf("init datasource '%s' err : %v", ds, err)
		}
		_ormMaps.Store(ds, orm)
		if ds == cfgKeyDefault {
			_ormDB = orm
		}
	}

	if _migrator != nil {
		err := _migrator()
		if err != nil {
			return nil, merrors.Errorf("migrate error : %v", err)
		}
	}
	return _execute, nil
}

func bindConfig() (map[string]*Config, error) {
	ret := make(map[string]*Config)
	defaultCfg := new(Config)
	err := kboot.UnmarshalSubConfig(ModuleName, defaultCfg,
		kboot.MustBindEnv(cfgKeyDbDsn),
		kboot.MustBindEnv(cfgKeyDbDebug),
		kboot.MustBindEnv(cfgKeyDbType),
		kboot.MustBindEnv(cfgKeyDbTimezone),
		kboot.MustBindEnv(cfgKeyDbSlowThresholdMs),
	)
	if err != nil {
		return nil, err
	}
	defaultCfg.name = cfgKeyDefault
	ret[cfgKeyDefault] = defaultCfg
	dbSettings := kboot.GetViper().Sub(ModuleName).AllSettings()
	for key := range dbSettings {
		switch dbSettings[key].(type) {
		case map[string]interface{}:
			_, exist := ret[key]
			if exist {
				return nil, merrors.Errorf("duplicate db setting : %s", key)
			}
			kboot.GetTaggedZapLogger(ModuleName).Info("try parser db settings ...", zap.String("name", key))
			var dsCfg = new(Config)
			if err = kboot.UnmarshalSubConfig(fmt.Sprintf("%s.%s", ModuleName, key), dsCfg,
				kboot.MustBindEnv(cfgKeyDbDsn),
				kboot.MustBindEnv(cfgKeyDbDebug),
				kboot.MustBindEnv(cfgKeyDbType),
				kboot.MustBindEnv(cfgKeyDbTimezone),
				kboot.MustBindEnv(cfgKeyDbSlowThresholdMs),
			); err != nil {
				return nil, err
			}
			dsCfg.name = key
			ret[key] = dsCfg
		}
	}
	return ret, nil
}

func _execute(unit kboot.Unit) kboot.ExitResult {
	<-unit.Done()
	return kboot.ExitResult{
		Code:  0,
		Error: nil,
	}
}
