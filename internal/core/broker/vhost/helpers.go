package vhost

func convertToPositiveInt64(arg any) (int64, bool) {
	switch v := arg.(type) {
	case int64:
		if v <= 0 {
			return 0, false
		}
		return v, true
	case int32:
		if v <= 0 {
			return 0, false
		}
		return int64(v), true
	case int:
		if v <= 0 {
			return 0, false
		}
		return int64(v), true
	case float64:
		if v <= 0 {
			return 0, false
		}
		return int64(v), true
	default:
		return 0, false
	}
}

func parseMaxPriorityArgument(args map[string]interface{}) (uint8, bool) {
	maxPrio, ok := args["x-max-priority"]
	if !ok {
		return 0, false
	}

	value, ok := convertToPositiveInt64(maxPrio)
	if !ok && value <= 0 {
		return 0, false
	}
	if value > 255 {
		value = 255
	}

	return uint8(value), true
}
