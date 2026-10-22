package main

type jsValueKind int

const (
	jsKindInvalid jsValueKind = iota
	jsKindString
	jsKindCollection
)

// classifyJSType classifies a JavaScript value from its type name and shape.
//
// Strings use the compact character format. Collections are arrays and typed
// arrays. Plain objects and other types are rejected so Length is never called
// on them.
//
// Parameters:
//   - typeName: JavaScript type name (string, object, null, undefined, ...).
//   - isArray: True when Array.isArray is true.
//   - isTypedArray: True when ArrayBuffer.isView is true.
//
// Returns:
//   - jsValueKind: String, collection, or invalid.
func classifyJSType(typeName string, isArray, isTypedArray bool) jsValueKind {
	switch typeName {
	case "string":
		return jsKindString
	case "object":
		if isArray || isTypedArray {
			return jsKindCollection
		}

		return jsKindInvalid
	default:
		return jsKindInvalid
	}
}
