package main

const modelGroupLogKeySeparator = "\x00"

func modelGroupLogKey(model, group string) string {
	return model + modelGroupLogKeySeparator + group
}

func groupLogs(rows []LogRow) map[string][]LogRow {
	m := map[string][]LogRow{}
	for _, r := range rows {
		if r.ModelName != "" {
			m[r.ModelName] = append(m[r.ModelName], r)
		}
	}
	return m
}

func groupLogsByModelGroup(rows []LogRow) map[string][]LogRow {
	m := map[string][]LogRow{}
	for _, r := range rows {
		for _, g := range parseList(r.Group) {
			if r.ModelName != "" && g != "" {
				key := modelGroupLogKey(r.ModelName, g)
				m[key] = append(m[key], r)
			}
		}
	}
	return m
}
