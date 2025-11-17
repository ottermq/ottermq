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
