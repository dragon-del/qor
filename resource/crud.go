package resource

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/qor/qor"
	"github.com/qor/qor/utils"
	"github.com/qor/roles"
)

// ToPrimaryQueryParams to primary query params
func (res *Resource) ToPrimaryQueryParams(primaryValue string, context *qor.Context) (string, []string) {
	if primaryValue != "" {
		scope := context.GetDB().NewScope(res.Value)

		// multiple primary fields
		if len(res.PrimaryFields) > 1 {
			if primaryValues := strings.Split(primaryValue, ","); len(primaryValues) == len(res.PrimaryFields) {
				sqls := []string{}
				for _, field := range res.PrimaryFields {
					sqls = append(sqls, fmt.Sprintf("%v.%v = ?", scope.QuotedTableName(), scope.Quote(field.DBName)))
				}

				return strings.Join(sqls, " AND "), primaryValues
			}
		}

		// fallback to first configured primary field
		if len(res.PrimaryFields) > 0 {
			return fmt.Sprintf("%v.%v = ?", scope.QuotedTableName(), scope.Quote(res.PrimaryFields[0].DBName)), []string{primaryValue}
		}

		// if no configured primary fields found
		if primaryField := scope.PrimaryField(); primaryField != nil {
			return fmt.Sprintf("%v.%v = ?", scope.QuotedTableName(), scope.Quote(primaryField.DBName)), []string{primaryValue}
		}
	}

	return "", []string{}
}

// ToPrimaryQueryParamsFromMetaValue to primary query params from meta values
func (res *Resource) ToPrimaryQueryParamsFromMetaValue(metaValues *MetaValues, context *qor.Context) (string, []string) {
	var (
		sqls, primaryValues []string
		scope               = context.GetDB().NewScope(res.Value)
	)

	if metaValues != nil {
		for _, field := range res.PrimaryFields {
			if metaField := metaValues.Get(field.Name); metaField != nil {
				sqls = append(sqls, fmt.Sprintf("%v.%v = ?", scope.QuotedTableName(), scope.Quote(field.DBName)))
				primaryValues = append(primaryValues, utils.ToString(metaField.Value))
			}
		}
	}

	return strings.Join(sqls, " AND "), primaryValues
}

func (res *Resource) findOneHandler(result interface{}, metaValues *MetaValues, context *qor.Context) error {
	if res.HasPermission(roles.Read, context) {
		var (
			primaryQuerySQL string
			primaryParams   []string
		)

		if metaValues == nil {
			primaryQuerySQL, primaryParams = res.ToPrimaryQueryParams(context.ResourceID, context)
		} else {
			primaryQuerySQL, primaryParams = res.ToPrimaryQueryParamsFromMetaValue(metaValues, context)
		}

		if primaryQuerySQL != "" {
			if metaValues != nil {
				if destroy := metaValues.Get("_destroy"); destroy != nil {
					if fmt.Sprint(destroy.Value) != "0" && res.HasPermission(roles.Delete, context) {
						context.GetDB().Delete(result, append([]string{primaryQuerySQL}, primaryParams...))
						return ErrProcessorSkipLeft
					}
				}
			}
			return context.GetDB().First(result, append([]string{primaryQuerySQL}, primaryParams...)).Error
		}

		return errors.New("failed to find")
	}
	return roles.ErrPermissionDenied
}

func (res *Resource) findManyHandler(result interface{}, context *qor.Context) error {
	if res.HasPermission(roles.Read, context) {
		db := context.GetDB()
		if _, ok := db.Get("qor:getting_total_count"); ok {
			return context.GetDB().Count(result).Error
		}
		return context.GetDB().Set("gorm:order_by_primary_key", "DESC").Find(result).Error
	}

	return roles.ErrPermissionDenied
}

func (res *Resource) saveHandler(result interface{}, context *qor.Context) error {
	if (context.GetDB().NewScope(result).PrimaryKeyZero() &&
		res.HasPermission(roles.Create, context)) || // has create permission
		res.HasPermission(roles.Update, context) { // has update permission
		return context.GetDB().Save(result).Error
	}
	return roles.ErrPermissionDenied
}

func (res *Resource) deleteHandler(result interface{}, context *qor.Context) error {
	if res.HasPermission(roles.Delete, context) {
		return gorm.ErrRecordNotFound
	}
	return roles.ErrPermissionDenied
}

// CallFindOne call find one method
func (res *Resource) CallFindOne(result interface{}, metaValues *MetaValues, context *qor.Context) error {
	return res.FindOneHandler(result, metaValues, context)
}

// CallFindMany call find many method
func (res *Resource) CallFindMany(result interface{}, context *qor.Context) error {
	return res.FindManyHandler(result, context)
}

// CallSave call save method
func (res *Resource) CallSave(result interface{}, context *qor.Context) error {
	return res.SaveHandler(result, context)
}

// CallDelete call delete method
func (res *Resource) CallDelete(result interface{}, context *qor.Context) error {
	return res.DeleteHandler(result, context)
}
