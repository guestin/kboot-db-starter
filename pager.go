package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/ooopSnake/assert.go"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type PageRequest struct {
	Page     *int `json:"page" query:"page" form:"page" path:"page" form:"page" validate:"omitempty,gt=0"`
	PageSize *int `json:"pageSize" query:"pageSize" form:"pageSize" path:"pageSize" form:"pageSize" validate:"omitempty,gt=0"`

	Begin int64  `json:"begin" query:"begin" form:"begin" path:"begin" form:"begin" validate:"gte=0"`
	End   *int64 `json:"end" query:"end" form:"end" path:"end" form:"end" validate:"omitempty,gtfield=Begin"`

	Key string `json:"key" query:"key" form:"key" path:"key" form:"key"`

	OrderBy string `json:"orderBy" query:"orderBy" form:"orderBy" path:"orderBy" form:"orderBy"`

	Order string `json:"order" query:"order" form:"order" path:"order" form:"order" validate:"omitempty,oneof=ASC DESC"`
}

func (this PageRequest) PageV() int {
	if this.Page != nil && *this.Page > 0 {
		return *this.Page
	}
	return 1
}

func (this PageRequest) PageSizeV() int {
	if this.PageSize != nil && *this.PageSize > 0 {
		return *this.PageSize
	}
	return 10
}

func (this PageRequest) BeginV() int64 {
	if this.Begin > 0 {
		return this.Begin
	}
	return 0
}

func (this PageRequest) EndV() int64 {
	if this.End != nil && *this.End > 0 {
		return *this.End
	}
	return 0
}

func (this PageRequest) OrderV() string {
	if this.Order != "" {
		return this.Order
	}
	return "ASC"
}

func (this PageRequest) Offset() int {
	return (this.PageV() - 1) * this.PageSizeV()
}

func (this PageRequest) Limit() int {
	return this.PageSizeV()
}

func (this PageRequest) BuildResponse(results interface{}) *PageResponse {
	return &PageResponse{
		Total:    0,
		Page:     this.PageV(),
		PageSize: this.PageSizeV(),
		Results:  results,
	}
}

type PageResponse struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	Results  interface{} `json:"results"`
}

type (
	Option interface {
		apply(ctx *pageCtx)
	}
	pageCtx struct {
		tx              *gorm.DB
		beginEndCol     string
		keyFuzzyCols    []string
		orderColsMap    map[string]string
		resultConverter resultConverterFunc
	}
	resultConverterFunc func(src interface{}) interface{}
	pageOptionFunc      func(ctx *pageCtx)
)

func (f pageOptionFunc) apply(ctx *pageCtx) {
	f(ctx)
}

func WithBeginEndCol(col string) Option {
	return pageOptionFunc(func(ctx *pageCtx) {
		ctx.beginEndCol = col
	})
}

func WithKeyFuzzyCol(col string) Option {
	return pageOptionFunc(func(ctx *pageCtx) {
		ctx.keyFuzzyCols = append(ctx.keyFuzzyCols, col)
	})
}

func WithKeyFuzzyCols(all ...string) Option {
	return pageOptionFunc(func(ctx *pageCtx) {
		ctx.keyFuzzyCols = all
	})
}

func WithOrderCol(param, col string) Option {
	return pageOptionFunc(func(ctx *pageCtx) {
		ctx.orderColsMap[param] = col
	})
}

func WithOrderCols(all map[string]string) Option {
	return pageOptionFunc(func(ctx *pageCtx) {
		ctx.orderColsMap = all
	})
}

func WithWhere(query interface{}, args ...interface{}) Option {
	return pageOptionFunc(func(ctx *pageCtx) {
		ctx.tx.Where(query, args...)
	})
}

func WithOrder(order interface{}) Option {
	return pageOptionFunc(func(ctx *pageCtx) {
		ctx.tx.Order(order)
	})
}

func WithResultConverter(fn resultConverterFunc) Option {
	return pageOptionFunc(func(ctx *pageCtx) {
		ctx.resultConverter = fn
	})
}

func PageQuery[T schema.Tabler](tx *gorm.DB, page PageRequest, m T, opts ...Option) (*PageResponse, error) {
	assert.Must(tx != nil, "tx must not be nil").Panic()
	ctx := &pageCtx{
		tx:              tx,
		beginEndCol:     "created_at",
		keyFuzzyCols:    make([]string, 0),
		orderColsMap:    make(map[string]string),
		resultConverter: nil,
	}
	for _, opt := range opts {
		if opt != nil {
			opt.apply(ctx)
		}
	}
	if page.BeginV() > 0 {
		ctx.tx = ctx.tx.Where(fmt.Sprintf("%s >= ?", ctx.beginEndCol), time.Unix(page.BeginV(), 0))
	}
	if page.EndV() > 0 {
		ctx.tx = ctx.tx.Where(fmt.Sprintf("%s <= ?", ctx.beginEndCol), time.Unix(page.EndV(), 0))
	}
	if len(page.OrderBy) > 0 {
		//check order
		colLimit := ctx.orderColsMap
		orderCol, ok := colLimit[page.OrderBy]
		if !ok {
			return nil, errors.Errorf("orderBy '%s' not allowed , must be one of [%s]", page.OrderBy,
				mkArrayString(colLimit))
		}
		ctx.tx = ctx.tx.Order(fmt.Sprintf("%s %s", orderCol, page.OrderV()))
	} else {
		//default order by begin end filter column desc
		ctx.tx = ctx.tx.Order(fmt.Sprintf("%s DESC", ctx.beginEndCol))
	}
	if len(page.Key) > 0 {
		key := page.Key
		orCols := make([]string, 0)
		args := make([]interface{}, 0)
		for _, col := range ctx.keyFuzzyCols {
			orCols = append(orCols, fmt.Sprintf("%s LIKE ? ", col))
			args = append(args, "%"+key+"%")
		}
		if len(orCols) > 0 {
			orQueryStr := fmt.Sprintf("(%s)", strings.Join(orCols, " OR "))
			ctx.tx = ctx.tx.Where(orQueryStr, args...)
		}
	}
	//mType := reflect.TypeOf(m)
	//if mType.Kind() != reflect.Ptr || mType.Elem().Kind() != reflect.Struct {
	//	return nil, errors.Errorf("m must be a pointer to a struct")
	//}
	//dbResults := reflect.Zero(reflect.SliceOf(mType)).Interface()
	dbResults := make([]T, 0)
	//dbResults := make([]interface{}, 0)
	total := int64(0)
	err := ctx.tx.Offset(-1).
		Model(m).
		Limit(-1).
		Count(&total).
		Offset(page.Offset()).
		Limit(page.Limit()).
		Find(&dbResults).Error
	if err != nil {
		return nil, err
	}
	//dbResultsVal := reflect.ValueOf(dbResults)
	resp := &PageResponse{
		Total:    total,
		Page:     page.PageV(),
		PageSize: page.PageSizeV(),
		Results:  nil,
	}
	if ctx.resultConverter != nil {
		resultsCvt := make([]interface{}, 0)
		for i := range len(dbResults) {
			//for i := range dbResultsVal.Len() {
			//resultsCvt = append(resultsCvt, ctx.resultConverter(dbResultsVal.Index(i).Interface()))
			resultsCvt = append(resultsCvt, ctx.resultConverter(dbResults[i]))
		}
		resp.Results = resultsCvt
	} else {
		resp.Results = dbResults
	}
	return resp, nil
}

func mkArrayString(names map[string]string) string {
	dbStrElements := make([]string, 0, len(names))
	for k := range names {
		dbStrElements = append(dbStrElements, fmt.Sprintf("'%s'", k))
	}
	return strings.Join(dbStrElements, ",")
}
