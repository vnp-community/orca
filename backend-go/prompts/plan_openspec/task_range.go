package plan_openspec

// TaskRange is the number of tasks a Request of this size should be split into, inside the project's total limit.
// Size is the domain's string value: "S", "M" or "L"; any other value gets the medium range.
func TaskRange(size string, maxTasksTotal int) (minTasks, maxTasks int) {
	switch size {
	case "S":
		minTasks, maxTasks = 1, 5
	case "L":
		minTasks, maxTasks = 6, 30
	default:
		minTasks, maxTasks = 3, 12
	}
	if maxTasks > maxTasksTotal {
		maxTasks = maxTasksTotal
	}
	if minTasks > maxTasks {
		minTasks = maxTasks
	}
	return minTasks, maxTasks
}
