package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func main() {
	// Parse command line arguments
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -input <html-file> -output <pdf-file>\n", os.Args[0])
		flag.PrintDefaults()
	}

	inputFile := flag.String("input", "", "Path to the input HTML file (required)")
	outputFile := flag.String("output", "", "Path for the output PDF file (required)")
	landscape := flag.Bool("landscape", false, "Set page orientation to landscape")
	paperSize := flag.String("paper", "A4", "Paper size (A4, Letter, Legal, etc.)")
	scale := flag.Float64("scale", 1.0, "Scale factor for rendering (default: 1.0)")
	printBackground := flag.Bool("background", true, "Print background colors and images")
	browserPath := flag.String("browser", "", "Path to Chrome/Chromium executable (for airgapped environments)")
	noDownload := flag.Bool("no-download", false, "Prevent automatic browser download (for airgapped environments)")
	timeout := flag.Int("timeout", 60, "Timeout in seconds for the conversion process")
	flag.Parse()

	// Check if required flags are provided
	if *inputFile == "" || *outputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Check if input file exists
	if _, err := os.Stat(*inputFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Input file '%s' not found.\n", *inputFile)
		os.Exit(1)
	}

	// Make sure output directory exists
	outputDir := filepath.Dir(*outputFile)
	if outputDir != "" && outputDir != "." {
		if _, err := os.Stat(outputDir); os.IsNotExist(err) {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
				os.Exit(1)
			}
		}
	}

	// Set a timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Create a channel to signal completion
	done := make(chan error, 1)

	go func() {
		done <- convertHTMLToPDF(ctx, *inputFile, *outputFile, PDFOptions{
			Landscape:       *landscape,
			PaperSize:       *paperSize,
			Scale:           *scale,
			PrintBackground: *printBackground,
			BrowserPath:     *browserPath,
			NoDownload:      *noDownload,
		})
	}()

	// Wait for completion or timeout
	select {
	case err := <-done:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error converting HTML to PDF: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully converted '%s' to '%s'\n", *inputFile, *outputFile)
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "Operation timed out after %d seconds\n", *timeout)
		os.Exit(1)
	}
}

// PDFOptions holds configuration for PDF generation
type PDFOptions struct {
	Landscape       bool
	PaperSize       string
	Scale           float64
	PrintBackground bool
	BrowserPath     string
	NoDownload      bool
}

// floatPtr returns a pointer to the provided float64 value
func floatPtr(f float64) *float64 {
	return &f
}

// boolPtr returns a pointer to the provided bool value
func boolPtr(b bool) *bool {
	return &b
}

func convertHTMLToPDF(ctx context.Context, htmlPath, pdfPath string, options PDFOptions) error {
	var l *launcher.Launcher
	var launchURL string

	// Check if browser path was provided
	if options.BrowserPath != "" {
		// Use provided browser path
		fmt.Printf("Using browser at: %s\n", options.BrowserPath)
		l = launcher.New().Bin(options.BrowserPath).Headless(true)
		launchURL = l.MustLaunch()
	} else {
		// Create browser launcher with auto-download capability
		l = launcher.New().Headless(true)

		// Disable auto-download if requested
		if options.NoDownload {
			fmt.Println("Auto-download disabled, searching for local browser installation...")

			// Try to find local Chrome installations
			paths := []string{
				// macOS
				"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
				"/Applications/Chromium.app/Contents/MacOS/Chromium",
				// Linux
				"/usr/bin/google-chrome",
				"/usr/bin/chromium",
				"/usr/bin/chromium-browser",
				// Windows
				`C:\Program Files\Google\Chrome\Application\chrome.exe`,
				`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
				`C:\Program Files\Chromium\Application\chrome.exe`,
				`C:\Program Files (x86)\Chromium\Application\chrome.exe`,
			}

			found := false
			for _, path := range paths {
				if _, err := os.Stat(path); err == nil {
					fmt.Printf("Found browser at: %s\n", path)
					l = launcher.New().Bin(path).Headless(true)
					found = true
					break
				}
			}

			// Try using launcher.LookPath as a last resort
			if !found {
				browserPath, exists := launcher.LookPath()
				if exists {
					fmt.Printf("Found browser using LookPath at: %s\n", browserPath)
					l = launcher.New().Bin(browserPath).Headless(true)
					found = true
				}
			}

			if !found {
				return fmt.Errorf("no browser download allowed and no local browser found")
			}

			launchURL = l.MustLaunch()
		} else {
			// Normal mode with auto-download
			launchURL = l.MustLaunch()
		}
	}

	// Add cleanup to ensure temporary files are removed
	defer l.Cleanup()

	// Create browser with context
	browser := rod.New().Context(ctx).ControlURL(launchURL).MustConnect()

	// Ensure browser is closed when function returns
	defer func() {
		err := browser.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error closing browser: %v\n", err)
		}
	}()

	// Create page
	page := browser.MustPage()

	// Load HTML file with file:// protocol
	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	fileURL := "file://" + absPath
	if !strings.HasPrefix(fileURL, "file:///") {
		fileURL = "file:///" + strings.TrimPrefix(fileURL, "file://")
	}

	if err := page.Navigate(fileURL); err != nil {
		return fmt.Errorf("failed to navigate to file: %w", err)
	}
	page.MustWaitLoad()

	// Wait for any pending network requests to complete
	page.MustWaitIdle()

	// Set paper size
	var paperWidth float64 = 8.5   // Default width for Letter
	var paperHeight float64 = 11.0 // Default height for Letter

	switch strings.ToUpper(options.PaperSize) {
	case "A4":
		paperWidth = 8.27
		paperHeight = 11.69
	case "LETTER":
		paperWidth = 8.5
		paperHeight = 11.0
	case "LEGAL":
		paperWidth = 8.5
		paperHeight = 14.0
	case "TABLOID", "LEDGER":
		paperWidth = 11.0
		paperHeight = 17.0
	case "A3":
		paperWidth = 11.69
		paperHeight = 16.54
	case "A5":
		paperWidth = 5.83
		paperHeight = 8.27
	}

	// Create PDF printing options with proper pointer values
	printOpts := &proto.PagePrintToPDF{
		Landscape:         options.Landscape,
		PrintBackground:   options.PrintBackground,
		Scale:             floatPtr(options.Scale),
		PaperWidth:        floatPtr(paperWidth),
		PaperHeight:       floatPtr(paperHeight),
		MarginTop:         floatPtr(0.4),
		MarginBottom:      floatPtr(0.4),
		MarginLeft:        floatPtr(0.4),
		MarginRight:       floatPtr(0.4),
		PreferCSSPageSize: true,
	}

	// Generate PDF
	pdfData, err := page.PDF(printOpts)
	if err != nil {
		return fmt.Errorf("failed to generate PDF: %w", err)
	}

	// Write PDF to file
	file, err := os.Create(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, pdfData)
	if err != nil {
		return fmt.Errorf("failed to write PDF data: %w", err)
	}

	return nil
}
