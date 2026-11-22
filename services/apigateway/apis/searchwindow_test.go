package apis

import "testing"

// 时间窗钳制：to 不能超前、from 不能缺省成 0、窗口不能无限大。
func TestNormalizeSearchWindow(t *testing.T) {
	const now int64 = 1762900000000
	const day int64 = 24 * 60 * 60 * 1000

	cases := []struct {
		name     string
		from, to int64
		wantFrom int64
		wantTo   int64
	}{
		{"都缺省用最近 7 天", 0, 0, now - 7*day, now},
		{"未来时间被钳到当前", 0, now + day, now - 7*day, now},
		{"from 晚于 to 时回落默认窗口", now, now - day, now - day - 7*day + day - day, now - day},
		{"正常窗口保持不变", now - 3*day, now, now - 3*day, now},
		{"超过 30 天被截断", now - 60*day, now, now - 30*day, now},
		{"只给 from 时 to 用当前", now - day, 0, now - day, now},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotFrom, gotTo := normalizeSearchWindow(tc.from, tc.to, now)
			if gotTo != tc.wantTo {
				t.Fatalf("to = %d, want %d", gotTo, tc.wantTo)
			}
			if gotFrom != tc.wantFrom {
				t.Fatalf("from = %d, want %d", gotFrom, tc.wantFrom)
			}
			if gotTo-gotFrom > searchMaxWindowMillis {
				t.Fatalf("窗口 %d 超过上限 %d", gotTo-gotFrom, searchMaxWindowMillis)
			}
		})
	}
}
