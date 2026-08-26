package videotranscript

import "testing"

func TestExtractShareURL(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "纯 URL 原样返回",
			input: "https://v.douyin.com/abc123",
			want:  "https://v.douyin.com/abc123",
		},
		{
			name:  "抖音口令嵌入短链",
			input: "5.84 :1pm 01/05 v@f.Bg xFU:/ 普通人怎样白手起家 https://v.douyin.com/iRnNwYFK/",
			want:  "https://v.douyin.com/iRnNwYFK/",
		},
		{
			name:  "口令无 URL 返回空",
			input: "5.84 :1pm 01/05 v@f.Bg xFU:/ 普通人怎样白手起家",
			want:  "",
		},
		{
			name:  "B 站链接嵌入文本",
			input: "看看这个视频 https://www.bilibili.com/video/BV1xx411c7mD 很有意思",
			want:  "https://www.bilibili.com/video/BV1xx411c7mD",
		},
		{
			name:  "尾部标点去除",
			input: "https://v.douyin.com/abc。",
			want:  "https://v.douyin.com/abc",
		},
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
		{
			name:  "快手口令",
			input: "快手看大片 https://v.kuaishou.com/xyz123 复制此链接",
			want:  "https://v.kuaishou.com/xyz123",
		},
		{
			name:  "多个 URL 取第一个",
			input: "https://a.com/1 https://b.com/2",
			want:  "https://a.com/1",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractShareURL(c.input)
			if got != c.want {
				t.Errorf("extractShareURL(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}
