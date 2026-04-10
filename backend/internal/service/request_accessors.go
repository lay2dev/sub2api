package service

import "reflect"

type requestValueGetter interface {
	Get(key string) (value any, exists bool)
}

type requestHeaderGetter interface {
	GetHeader(key string) string
}

type requestHeaderValueGetter interface {
	requestValueGetter
	requestHeaderGetter
}

func isNilRequestValueGetter(getter requestValueGetter) bool {
	if getter == nil {
		return true
	}
	v := reflect.ValueOf(getter)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

func isNilRequestHeaderGetter(getter requestHeaderGetter) bool {
	if getter == nil {
		return true
	}
	v := reflect.ValueOf(getter)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

func isNilRequestHeaderValueGetter(getter requestHeaderValueGetter) bool {
	if getter == nil {
		return true
	}
	v := reflect.ValueOf(getter)
	return v.Kind() == reflect.Pointer && v.IsNil()
}
