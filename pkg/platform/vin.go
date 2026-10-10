package platform

import "fmt"

// ValidateVIN 校验车辆识别码。实验环境只执行 ISO 3779 的字面约束：
// 17 位，字母数字，且不包含 I、O、Q（避免和 1、0 混淆）。
// 完整校验位算法留给车籍主数据系统，不放在上报热路径上。
func ValidateVIN(vin string) error {
	if len(vin) != 17 {
		return fmt.Errorf("vin 必须是 17 位")
	}
	for _, r := range vin {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'A' && r <= 'Z' && r != 'I' && r != 'O' && r != 'Q':
		default:
			return fmt.Errorf("vin 含有非法字符 %q", r)
		}
	}
	return nil
}
