package internal

import (
	"fmt"
	"reflect"
	"strings"
)

type ValidationError struct {
	MissingFields []string
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("missing fields: %s", strings.Join(v.MissingFields, ", "))
}

func (v ValidationError) IsEmpty() bool {
	return len(v.MissingFields) == 0
}

// Validate searches for validate struct tags and performs the validations
// requested by them
func Validate(obj interface{}) error {
	errs := ValidationError{}

	val := reflect.ValueOf(obj)
	typ := reflect.TypeOf(obj)

	for i := 0; i < val.NumField(); i++ {
		fieldVal := val.Field(i)
		fieldTyp := typ.Field(i)

		op := fieldTyp.Tag.Get("validate")
		switch op {
		case "presence":
			if fieldVal.Kind() == reflect.Slice {
				if fieldVal.Len() == 0 {
					errs.MissingFields = append(errs.MissingFields, fieldTyp.Name)
				}
			} else if fieldVal.IsZero() {
				errs.MissingFields = append(errs.MissingFields, fieldTyp.Name)
			}
		}
	}

	if errs.IsEmpty() {
		return nil
	}

	return errs
}
