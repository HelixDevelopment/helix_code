// Package logo provides image processing utilities for brand asset management in the HelixCode platform.
//
// The logo package handles logo processing operations including color extraction, icon generation,
// ASCII art creation, and theme file production. It enables consistent branding across different
// platforms and output formats by deriving color schemes and assets from source logo images.
//
// # Features
//
// The package provides the following capabilities:
//
//   - Color Extraction: Analyze logo images to extract dominant colors
//   - Icon Generation: Create multi-size icons for different platforms
//   - ASCII Art: Convert logos to ASCII art for terminal display
//   - Theme Generation: Produce CSS and Go theme files from color schemes
//
// # Color Scheme
//
// The ColorScheme type represents the brand color palette:
//
//	type ColorScheme struct {
//	    Primary    string // Main brand color (e.g., "#2E86AB")
//	    Secondary  string // Secondary accent color
//	    Accent     string // Highlight color
//	    Text       string // Text color
//	    Background string // Background color
//	}
//
// # Basic Usage
//
// Creating a processor and extracting colors:
//
//	processor := logo.NewLogoProcessor("/path/to/logo.png", "/output/dir")
//
//	// Extract dominant colors from the logo
//	err := processor.ExtractColors()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access the extracted colors
//	fmt.Printf("Primary: %s\n", processor.Colors.Primary)
//	fmt.Printf("Accent: %s\n", processor.Colors.Accent)
//
// # Icon Generation
//
// Generate icons in multiple sizes for different platforms:
//
//	processor := logo.NewLogoProcessor(sourcePath, outputDir)
//
//	err := processor.GenerateIcons()
//	// Generates:
//	// - icons/icon-16x16.png
//	// - icons/icon-32x32.png
//	// - icons/icon-48x48.png
//	// - icons/icon-64x64.png
//	// - icons/icon-128x128.png
//	// - icons/icon-256x256.png
//	// - icons/icon-512x512.png
//
// # ASCII Art Generation
//
// Create ASCII art representation for terminal display:
//
//	asciiArt, err := processor.GenerateASCIIArt()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(asciiArt)
//
// Output example:
//
//	       .::-=+*##%%@@@@%%##*+=:-:.
//	    .:-+#%%@@@@@@@@@@@@@@@@@%%#+-.
//	   :+#@@@@@@@@@@@@@@@@@@@@@@@@@@#:
//	  -#@@@@@@@@@@@@@@@@@@@@@@@@@@@@#-
//
// # Theme File Generation
//
// Generate theme files for CSS and Go:
//
//	err := processor.GenerateThemeFiles()
//
// This creates CSS variables:
//
//	:root {
//	  --primary-color: #2E86AB;
//	  --secondary-color: #A23B72;
//	  --accent-color: #F18F01;
//	  --text-color: #2D3047;
//	  --background-color: #F5F5F5;
//	}
//
// And Go constants:
//
//	package theme
//
//	const (
//	    PrimaryColor    = "#2E86AB"
//	    SecondaryColor  = "#A23B72"
//	    AccentColor     = "#F18F01"
//	    TextColor       = "#2D3047"
//	    BackgroundColor = "#F5F5F5"
//	)
//
// # Saving Color Scheme
//
// Export the color scheme as JSON:
//
//	err := processor.SaveColorScheme()
//	// Creates: colors/color-scheme.json
//
// # Default Colors
//
// The processor initializes with HelixCode's default brand colors:
//
//   - Primary: #2E86AB (Deep blue)
//   - Secondary: #A23B72 (Purple accent)
//   - Accent: #F18F01 (Orange highlight)
//   - Text: #2D3047 (Dark text)
//   - Background: #F5F5F5 (Light background)
//
// These defaults are used when color extraction cannot determine suitable colors.
//
// # Output Structure
//
// The processor generates files in this structure:
//
//	output/
//	├── icons/
//	│   ├── icon-16x16.png
//	│   ├── icon-32x32.png
//	│   └── ...
//	├── colors/
//	│   ├── color-scheme.json
//	│   └── helix-theme.css
//	└── internal/theme/
//	    └── theme.go
//
// # Dependencies
//
// The package uses github.com/nfnt/resize for high-quality image resizing
// with Lanczos3 resampling algorithm.
package logo
