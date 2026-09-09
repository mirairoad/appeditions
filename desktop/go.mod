// The desktop shell is a nested module because it needs cgo and a webview
// binding, and the application's own go.mod should not carry either. A machine
// without WebKitGTK still builds everything else.
module github.com/mirairoad/appeditions/desktop

go 1.25.0

require (
	github.com/mirairoad/appeditions v0.0.0
	github.com/mirairoad/howl-go v0.3.0
	github.com/mirairoad/howl-go/desktop v0.1.0
)

require (
	github.com/Oudwins/tailwind-merge-go v0.2.2 // indirect
	github.com/a-h/templ v0.3.1020 // indirect
	github.com/axadrn/shadcn-templ/v2 v2.0.0-beta.3 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fogleman/gg v1.3.0 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/webview/webview_go v0.0.0-20240831120633-6173450d4dd6 // indirect
	golang.org/x/image v0.45.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	modernc.org/libc v1.75.6 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.58.0 // indirect
)

replace github.com/mirairoad/appeditions => ..
