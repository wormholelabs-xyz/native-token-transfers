package main

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSameShapeValueTypesAreNotConvertible pins that no explicit conversion compiles between the
// single-string-field value types.
func TestSameShapeValueTypesAreNotConvertible(t *testing.T) {
	valueTypes := []reflect.Type{
		reflect.TypeOf(managerAddressHex{}),
		reflect.TypeOf(vaaDigestHex{}),
		reflect.TypeOf(claimedRootDigestHex{}),
		reflect.TypeOf(nttManagerAddressValue{}),
		reflect.TypeOf(digestNibblePrefixValue{}),
		reflect.TypeOf(replayNamespace{}),
		reflect.TypeOf(instrumentIDText{}),
	}
	for _, valueType := range valueTypes {
		require.Equal(t, reflect.Struct, valueType.Kind(), valueType.String())
		require.Equal(t, 1, valueType.NumField(), valueType.String())
		require.Equal(t, reflect.String, valueType.Field(0).Type.Kind(), valueType.String())
	}
	for _, sourceType := range valueTypes {
		for _, targetType := range valueTypes {
			if sourceType == targetType {
				continue
			}
			assert.False(t, sourceType.ConvertibleTo(targetType), "%s converts to %s", sourceType, targetType)
		}
	}
}
