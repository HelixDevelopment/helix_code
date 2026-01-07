# Logo Package

The logo package provides image processing utilities for extracting color schemes, generating icons, creating ASCII art, and producing theme files from HelixCode logos and branding assets.

## Overview

This package handles brand asset management by:
- Extracting dominant colors from logo images
- Generating multi-size icons for different platforms
- Converting logos to ASCII art for terminal display
- Producing CSS and Go theme files

## Types

### ColorScheme

Represents the extracted color palette:

```go
type ColorScheme struct {
    Primary    string `json:"primary"`    // Main brand color
    Secondary  string `json:"secondary"`  // Secondary color
    Accent     string `json:"accent"`     // Accent/highlight color
    Text       string `json:"text"`       // Text color
    Background string `json:"background"` // Background color
}
```

### LogoProcessor

Main processor for logo operations:

```go
type LogoProcessor struct {
    SourcePath string       // Path to source logo image
    OutputDir  string       // Output directory for generated assets
    Colors     ColorScheme  // Extracted/configured color scheme
}
```

## Usage

### Creating a Processor

```go
processor := logo.NewLogoProcessor(
    "/path/to/logo.png",
    "/path/to/output",
)
```

### Extracting Colors

Analyzes the logo image to extract dominant colors:

```go
err := processor.ExtractColors()
if err != nil {
    log.Fatal(err)
}

// Access extracted colors
fmt.Println("Primary:", processor.Colors.Primary)
fmt.Println("Accent:", processor.Colors.Accent)
```

### Generating ASCII Art

Creates ASCII art representation for terminal display:

```go
asciiArt, err := processor.GenerateASCIIArt()
if err != nil {
    log.Fatal(err)
}
fmt.Println(asciiArt)
```

Output example:
```
       .::-=+*##%%@@@@%%##*+=:-:.
    .:-+#%%@@@@@@@@@@@@@@@@@%%#+-.
   :+#@@@@@@@@@@@@@@@@@@@@@@@@@@#:
  -#@@@@@@@@@@@@@@@@@@@@@@@@@@@@#-
```

### Generating Icons

Creates multiple icon sizes for different platforms:

```go
err := processor.GenerateIcons()
if err != nil {
    log.Fatal(err)
}
// Generates: icon-16x16.png through icon-512x512.png
```

Generated sizes:
- 16x16, 32x32, 48x48, 64x64, 128x128, 256x256, 512x512

### Saving Color Scheme

Exports color scheme as JSON:

```go
err := processor.SaveColorScheme()
if err != nil {
    log.Fatal(err)
}
// Creates: output/colors/color-scheme.json
```

### Generating Theme Files

Creates theme files for CSS and Go:

```go
err := processor.GenerateThemeFiles()
if err != nil {
    log.Fatal(err)
}
```

Generated files:
- `colors/helix-theme.css` - CSS variables and classes
- `internal/theme/theme.go` - Go constants

CSS output:
```css
:root {
  --primary-color: #2E86AB;
  --secondary-color: #A23B72;
  --accent-color: #F18F01;
  --text-color: #2D3047;
  --background-color: #F5F5F5;
}
```

Go output:
```go
package theme

const (
    PrimaryColor    = "#2E86AB"
    SecondaryColor  = "#A23B72"
    AccentColor     = "#F18F01"
    TextColor       = "#2D3047"
    BackgroundColor = "#F5F5F5"
)
```

## Default Colors

The processor uses HelixCode's default color palette:

| Color | Hex | Usage |
|-------|-----|-------|
| Primary | `#2E86AB` | Deep blue - main brand color |
| Secondary | `#A23B72` | Purple accent |
| Accent | `#F18F01` | Orange highlight |
| Text | `#2D3047` | Dark text |
| Background | `#F5F5F5` | Light background |

## Dependencies

- `github.com/nfnt/resize` - Image resizing with Lanczos resampling
- Standard library: `image`, `image/png`, `image/color`

## File Structure

Generated output structure:
```
output/
├── icons/
│   ├── icon-16x16.png
│   ├── icon-32x32.png
│   └── ...
├── colors/
│   ├── color-scheme.json
│   └── helix-theme.css
```
