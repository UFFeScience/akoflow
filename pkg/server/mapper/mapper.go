package mapper

import "reflect"

func MapStructs(src, dst interface{}) {
	srcValue := reflect.ValueOf(src)
	dstValue := reflect.ValueOf(dst).Elem()

	for i := 0; i < srcValue.NumField(); i++ {
		srcField := srcValue.Field(i)
		dstField := dstValue.FieldByName(srcValue.Type().Field(i).Name)

		if dstField.IsValid() && dstField.CanSet() {
			switch srcField.Kind() {
			case reflect.Struct:
				MapStructs(srcField.Interface(), dstField.Addr().Interface())
			case reflect.Slice, reflect.Array:
				if dstField.Kind() == reflect.Slice || dstField.Kind() == reflect.Array {
					slice := reflect.MakeSlice(dstField.Type(), srcField.Len(), srcField.Cap())
					for j := 0; j < srcField.Len(); j++ {
						srcElem := srcField.Index(j)
						dstElem := slice.Index(j)

						if srcElem.Kind() == reflect.Struct {
							MapStructs(srcElem.Interface(), dstElem.Addr().Interface())
						} else if srcElem.Type() == dstElem.Type() {
							dstElem.Set(srcElem)
						}
					}
					dstField.Set(slice)
				}
			default:
				if srcField.Type() == dstField.Type() {
					dstField.Set(srcField)
				}
			}
		}
	}
}
