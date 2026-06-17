package main

import "fufu/modelcore"

const modelGroupLogKeySeparator = "\x00"

func modelGroupLogKey(model, group string) string {
	return modelcore.ModelGroupLogKey(model, group)
}

func groupLogs(rows []LogRow) map[string][]LogRow { return modelcore.GroupLogs(rows) }

func groupLogsByModelGroup(rows []LogRow) map[string][]LogRow {
	return modelcore.GroupLogsByModelGroup(rows)
}
