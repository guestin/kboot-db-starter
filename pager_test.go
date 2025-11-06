package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"
)

type user struct {
	UuidPriWithCreateDelAtBase
	Name string `gorm:"column:name"`
	Age  int    `gorm:"column:age"`
	Sex  string `gorm:"column:sex"`
}

func (*user) TableName() string {
	return "t_users"
}
func (*user) AfterFind(session *gorm.DB) (err error) {
	fmt.Println("AAA")
	return nil
}

func TestPageQuery(t *testing.T) {
	cfg := Config{
		name:     "test",
		Type:     DsTypeSqlLite,
		DSN:      "test.db",
		Debug:    true,
		Colorful: new(bool),
	}
	*cfg.Colorful = true
	ctx := context.WithValue(context.WithValue(context.Background(), CtxTraceIdKey, "1111111"), CtxTraceSkipKey, 1)
	orm, err := newORM(ctx, cfg, time.Local)
	if err != nil {
		t.Fatalf("new orm err %v", err)
	}
	defer func() {
		_ = os.Remove("test.db")
	}()

	err = orm.AutoMigrate(new(user))
	if err != nil {
		t.Fatalf("migrate err %v", err)
	}
	for i := 0; i < 30; i++ {
		orm.Create(&user{
			Name: fmt.Sprintf("user%d", i),
			Age:  18,
			Sex:  "",
		})
	}
	pageReq := PageRequest{}

	res, err := PageQuery[*user](orm, pageReq, new(user), WithResultConverter(func(src interface{}) interface{} {
		u := src.(*user)
		t.Log("user ", u.Name)
		return src
	}))
	if err != nil {
		t.Fatalf("query err %v", err)
	}
	t.Log(res)
}
