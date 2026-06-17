package main

import "fufu/modelcore"

const modelManualKeySeparator = "\x00"

func modelManualKey(siteName, model, group string) string {
	return modelcore.ModelManualKey(siteName, model, group)
}
