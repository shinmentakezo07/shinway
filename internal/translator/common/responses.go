package common

import "github.com/tidwall/sjson"

// SetResponsesToolCallIdentity writes a resolved Responses tool name and
// namespace. Stale namespace fields are removed when the resolved name has no
// namespace so tool-call identity stays consistent across conversions.
func SetResponsesToolCallIdentity(item []byte, name, namespace, itemPath string) []byte {
	namePath := "name"
	namespacePath := "namespace"
	if itemPath != "" {
		namePath = itemPath + ".name"
		namespacePath = itemPath + ".namespace"
	}
	item, _ = sjson.SetBytes(item, namePath, name)
	if namespace != "" {
		item, _ = sjson.SetBytes(item, namespacePath, namespace)
	} else {
		item, _ = sjson.DeleteBytes(item, namespacePath)
	}
	return item
}
