package main

const modelManualKeySeparator = "\x00"

func modelManualKey(siteName, model, group string) string {
	return siteName + modelManualKeySeparator + model + modelManualKeySeparator + group
}
