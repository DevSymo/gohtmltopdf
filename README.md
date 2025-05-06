# Go Html to PDF Convert

## _gohtmltopdf_

## Usage

`./gohtmltopdf`

Usage: ./gohtmltopdf -input <html-file> -output <pdf-file>
-background
Print background colors and images (default true)
-browser string
Path to Chrome/Chromium executable (for airgapped environments)
-input string
Path to the input HTML file (required)
-landscape
Set page orientation to landscape
-no-download
Prevent automatic browser download (for airgapped environments)
-output string
Path for the output PDF file (required)
-paper string
Paper size (A4, Letter, Legal, etc.) (default "A4")
-rod string
Set the default value of options used by rod.
-scale float
Scale factor for rendering (default: 1.0) (default 1)
-timeout int
Timeout in seconds for the conversion process (default 60)

## Build

`go build -o gohtmltopdf main.go`

## Example

- `./gohtmltopdf -input '/tmp/Report April 2025.html' -output ./report.pdf -scale 0.7 -no-download  `
- `./gohtmltopdf -input '/tmp/Report April 2025.html' -output ./report.pdf -scale 0.7 -browser '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'`
- `./gohtmltopdf -input '/tmp/Report April 2025.html' -output ./report.pdf -scale 0.7`

## License

MIT

**Free Software, Hell Yeah!**
