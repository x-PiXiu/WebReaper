package crawler

import (
	"testing"
)

func TestParseVideoID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name: "纯数字ID",
			url:  "7525538910311632128",
			want: "7525538910311632128",
		},
		{
			name: "标准链接",
			url:  "https://www.douyin.com/video/7525538910311632128",
			want: "7525538910311632128",
		},
		{
			name: "带参数链接",
			url:  "https://www.douyin.com/video/7525538910311632128?spm_id_from=333.1007.0.0",
			want: "7525538910311632128",
		},
		{
			name:    "无效链接",
			url:     "https://www.douyin.com/user/123",
			wantErr: true,
		},
		{
			name:    "空字符串",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVideoID(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVideoID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseVideoID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewVideoDownloader(t *testing.T) {
	d := NewVideoDownloader("test_cookie")
	if d.cookies != "test_cookie" {
		t.Errorf("cookies = %v, want test_cookie", d.cookies)
	}
	if d.userAgent == "" {
		t.Error("userAgent should not be empty")
	}
	if d.client == nil {
		t.Error("client should not be nil")
	}
}

func TestParseVideoID_ShortLink(t *testing.T) {
	// 短链接需要网络请求，这里只测试格式识别
	_, err := ParseVideoID("https://v.douyin.com/iF12345ABC/")
	// 短链接解析会失败（无网络），但不应该 panic
	if err != nil {
		t.Logf("短链接解析失败（预期）: %v", err)
	}
}
