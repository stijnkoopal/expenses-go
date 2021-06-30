package utils

import (
	"reflect"

	"github.com/almerlucke/go-iban/iban"
)

func TypeNameOf(v interface{}) string {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		rv = rv.Elem()
	}

	if rv.IsValid() {
		return rv.Type().Name()
	}
	return "Invalid"
}

func EmptyStringOrValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func EmptyIBANOrValue(ptr *iban.IBAN) iban.IBAN {
	if ptr == nil {
		return iban.IBAN{}
	}
	return *ptr
}
