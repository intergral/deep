/*
 * Copyright (C) 2023  Intergral GmbH
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package util

import (
	v1_common "github.com/intergral/deep/pkg/deeppb/common/v1"
	semconv "go.opentelemetry.io/collector/semconv/v1.9.0"
	"strconv"
)

func FindServiceName(attributes []*v1_common.KeyValue) (string, bool) {
	return FindAttributeValue(semconv.AttributeServiceName, attributes)
}

func FindAttributeValue(key string, attributes ...[]*v1_common.KeyValue) (string, bool) {
	for _, attrs := range attributes {
		for _, kv := range attrs {
			if key == kv.Key {
				return StringifyAnyValue(kv.Value), true
			}
		}
	}
	return "", false
}

func GetSpanMultiplier(ratioKey string, attributes []*v1_common.KeyValue) float64 {
	spanMultiplier := 1.0
	if ratioKey != "" {
		for _, kv := range attributes {
			if kv.Key == ratioKey {
				v := kv.Value.GetDoubleValue()
				if v > 0 {
					spanMultiplier = 1.0 / v
				}
			}
		}
	}
	return spanMultiplier
}

func StringifyAnyValue(anyValue *v1_common.AnyValue) string {
	switch anyValue.Value.(type) {
	case *v1_common.AnyValue_BoolValue:
		return strconv.FormatBool(anyValue.GetBoolValue())
	case *v1_common.AnyValue_IntValue:
		return strconv.FormatInt(anyValue.GetIntValue(), 10)
	case *v1_common.AnyValue_ArrayValue:
		arrStr := "["
		for _, v := range anyValue.GetArrayValue().Values {
			arrStr += StringifyAnyValue(v)
		}
		arrStr += "]"
		return arrStr
	case *v1_common.AnyValue_DoubleValue:
		return strconv.FormatFloat(anyValue.GetDoubleValue(), 'f', -1, 64)
	case *v1_common.AnyValue_KvlistValue:
		mapStr := "{"
		for _, kv := range anyValue.GetKvlistValue().Values {
			mapStr += kv.Key + ":" + StringifyAnyValue(kv.Value)
		}
		mapStr += "}"
		return mapStr
	case *v1_common.AnyValue_StringValue:
		return anyValue.GetStringValue()
	}

	return ""
}
