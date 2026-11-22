package configures

import "testing"

func TestGetAdminBindAddr(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		want       string
	}{
		{name: "未配置时默认回环", configured: "", want: "127.0.0.1"},
		{name: "只有空白时默认回环", configured: "   ", want: "127.0.0.1"},
		{name: "显式配置原样返回", configured: "10.0.0.5", want: "10.0.0.5"},
		{name: "显式配置去掉两端空白", configured: " 10.0.0.5 ", want: "10.0.0.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var conf ImConfig
			conf.AdminGateway.BindAddr = tc.configured
			if got := conf.GetAdminBindAddr(); got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}
