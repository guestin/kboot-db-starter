package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestPageQuery(t *testing.T) {
	orm, err := newORM(context.Background(), Config{
		name:  "test",
		Type:  DsTypeSqlLite,
		DSN:   "test.db",
		Debug: true,
	}, time.Local)
	if err != nil {
		t.Fatalf("new orm err %v", err)
	}
	defer func() {
		_ = os.Remove("test.db")
	}()
	type User struct {
		UuidPriWithCreateDelAtBase
		Name string `gorm:"column:name"`
		Age  int    `gorm:"column:age"`
		Sex  string `gorm:"column:sex"`
	}
	err = orm.AutoMigrate(new(User))
	if err != nil {
		t.Fatalf("migrate err %v", err)
	}
	for i := 0; i < 30; i++ {
		orm.Create(&User{
			Name: fmt.Sprintf("user%d", i),
			Age:  18,
			Sex:  "",
		})
	}
	pageReq := PageRequest{}

	res, err := PageQuery(orm, pageReq, new(User), WithResultConverter(func(src interface{}) interface{} {
		u := src.(*User)
		t.Log("user ", u.Name)
		return src
	}))
	if err != nil {
		t.Fatalf("query err %v", err)
	}
	t.Log(res)
}
