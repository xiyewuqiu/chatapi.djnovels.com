package services

import (
	"testing"
)

const oneDayMillis int64 = 24 * 60 * 60 * 1000

func TestRetentionFloorAt(t *testing.T) {
	const now int64 = 1762900000000

	cases := []struct {
		name string
		days int
		want int64
	}{
		{"未设置保留期即不设下界", 0, 0},
		{"负数按未设置处理", -5, 0},
		{"保留 1 天", 1, now - oneDayMillis},
		{"保留 7 天（默认）", 7, now - 7*oneDayMillis},
		{"保留 30 天（平台上限）", 30, now - 30*oneDayMillis},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := retentionFloorAt(tc.days, now); got != tc.want {
				t.Fatalf("retentionFloorAt(%d, %d) = %d, want %d", tc.days, now, got, tc.want)
			}
		})
	}
}

// 读路径的时间下界必须取"用户清除时间"与"租户保留期"的较大者：
// 取小值会让保留期失效，取大值会吞掉用户主动清除的语义。
func TestMaxCleanTime(t *testing.T) {
	cases := []struct {
		name      string
		cleanTime int64
		floor     int64
		want      int64
	}{
		{"两边都没有下界", 0, 0, 0},
		{"只有用户清除时间", 100, 0, 100},
		{"只有保留期", 0, 200, 200},
		{"保留期更晚", 100, 200, 200},
		{"用户清除时间更晚", 300, 200, 300},
		{"完全相等", 200, 200, 200},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maxCleanTime(tc.cleanTime, tc.floor); got != tc.want {
				t.Fatalf("maxCleanTime(%d, %d) = %d, want %d", tc.cleanTime, tc.floor, got, tc.want)
			}
		})
	}
}
