// qrcheck：用 gozxing 确定性解码图片中的二维码（本地验证工具）。
// 用法：go run ./cmd/qrcheck 文件1.png 文件2.png ...
// 对每张图依次尝试：整图解码 → 四象限裁剪解码 → 中心 60% 裁剪解码。
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

func tryDecode(img image.Image) (string, bool) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", false
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil || res == nil {
		return "", false
	}
	return res.GetText(), true
}

func sub(img image.Image, x0, y0, x1, y1 int) image.Image {
	b := img.Bounds()
	type subimager interface{ SubImage(r image.Rectangle) image.Image }
	if s, ok := img.(subimager); ok {
		return s.SubImage(image.Rect(x0, y0, x1, y1).Intersect(b))
	}
	return img
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: qrcheck <png文件>...")
		os.Exit(1)
	}
	fail := 0
	for _, path := range os.Args[1:] {
		f, err := os.Open(path)
		if err != nil {
			fmt.Printf("%s: 打开失败 %v\n", path, err)
			fail++
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			fmt.Printf("%s: 解码图片失败 %v\n", path, err)
			fail++
			continue
		}
		b := img.Bounds()
		w, h := b.Dx(), b.Dy()

		if text, ok := tryDecode(img); ok {
			fmt.Printf("%s: ✅ 整图即二维码 [%dx%d] → %s\n", path, w, h, text)
			continue
		}
		// 四象限 + 中心裁剪
		found := false
		crops := map[string]image.Rectangle{
			"左上":   image.Rect(b.Min.X, b.Min.Y, b.Min.X+w/2, b.Min.Y+h/2),
			"右上":   image.Rect(b.Min.X+w/2, b.Min.Y, b.Max.X, b.Min.Y+h/2),
			"左下":   image.Rect(b.Min.X, b.Min.Y+h/2, b.Min.X+w/2, b.Max.Y),
			"右下":   image.Rect(b.Min.X+w/2, b.Min.Y+h/2, b.Max.X, b.Max.Y),
			"中心60%": image.Rect(b.Min.X+w/5, b.Min.Y+h/5, b.Max.X-w/5, b.Max.Y-h/5),
		}
		for _, name := range []string{"左上", "右上", "左下", "右下", "中心60%"} {
			if text, ok := tryDecode(sub(img, crops[name].Min.X, crops[name].Min.Y, crops[name].Max.X, crops[name].Max.Y)); ok {
				fmt.Printf("%s: ✅ %s区域是二维码 [%dx%d] → %s\n", path, name, w, h, text)
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("%s: ❌ 未找到二维码 [%dx%d]\n", path, w, h)
			fail++
		}
	}
	os.Exit(min(fail, 1))
}
