package logo

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// createTestPNG creates a test PNG image with specific colors for testing
func createTestPNG(path string, width, height int, colors []color.Color) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill image with colors in stripes
	stripeHeight := height / len(colors)
	for i, c := range colors {
		for y := i * stripeHeight; y < (i+1)*stripeHeight; y++ {
			for x := 0; x < width; x++ {
				img.Set(x, y, c)
			}
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

func TestNewLogoProcessor(t *testing.T) {
	processor := NewLogoProcessor("test.png", "output")

	assert.NotNil(t, processor)
	assert.Equal(t, "test.png", processor.SourcePath)
	assert.Equal(t, "output", processor.OutputDir)
	assert.Equal(t, "#2E86AB", processor.Colors.Primary)
	assert.Equal(t, "#A23B72", processor.Colors.Secondary)
	assert.Equal(t, "#F18F01", processor.Colors.Accent)
}

func TestColorToHex(t *testing.T) {
	// Test with black
	hex := colorToHex(color.RGBA{0, 0, 0, 255})
	assert.Equal(t, "#000000", hex)

	// Test with white
	hex = colorToHex(color.RGBA{255, 255, 255, 255})
	assert.Equal(t, "#FFFFFF", hex)

	// Test with red
	hex = colorToHex(color.RGBA{255, 0, 0, 255})
	assert.Equal(t, "#FF0000", hex)
}

func TestGrayToASCII(t *testing.T) {
	// Test black (should be lightest char)
	char := grayToASCII(0)
	assert.Equal(t, " ", char)

	// Test white (should be darkest char)
	char = grayToASCII(255)
	assert.Equal(t, "@", char)

	// Test middle gray
	char = grayToASCII(128)
	assert.Equal(t, "=", char)
}

func TestSaveColorScheme(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logo_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	processor := NewLogoProcessor("test.png", tempDir)

	// Create colors directory
	colorsDir := filepath.Join(tempDir, "colors")
	err = os.MkdirAll(colorsDir, 0755)
	assert.NoError(t, err)

	err = processor.SaveColorScheme()
	assert.NoError(t, err)

	// Check if file was created
	colorFile := filepath.Join(tempDir, "colors", "color-scheme.json")
	_, err = os.Stat(colorFile)
	assert.NoError(t, err)
}

func TestGenerateThemeFiles(t *testing.T) {
	// Create a temp directory structure that mimics the project layout:
	// baseDir/
	//   assets/logo/  <- OutputDir
	//   internal/theme/  <- where Go file will be generated
	baseDir, err := os.MkdirTemp("", "logo_test")
	assert.NoError(t, err)
	defer os.RemoveAll(baseDir)

	// Create the expected directory structure
	outputDir := filepath.Join(baseDir, "assets", "logo")
	err = os.MkdirAll(outputDir, 0755)
	assert.NoError(t, err)

	// Create colors directory
	colorsDir := filepath.Join(outputDir, "colors")
	err = os.MkdirAll(colorsDir, 0755)
	assert.NoError(t, err)

	processor := NewLogoProcessor("test.png", outputDir)

	err = processor.GenerateThemeFiles()
	assert.NoError(t, err)

	// Check if CSS file was created
	cssFile := filepath.Join(outputDir, "colors", "helix-theme.css")
	_, err = os.Stat(cssFile)
	assert.NoError(t, err)

	// Check if Go file was created (path is relative: ../../internal/theme/theme.go)
	goFile := filepath.Join(baseDir, "internal", "theme", "theme.go")
	_, err = os.Stat(goFile)
	assert.NoError(t, err)

	// Check content
	content, err := os.ReadFile(goFile)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "PrimaryColor")
}

func TestExtractColors(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logo_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("extract colors from valid image", func(t *testing.T) {
		// Create a test image with distinct colors
		testImage := filepath.Join(tempDir, "test.png")
		testColors := []color.Color{
			color.RGBA{255, 0, 0, 255}, // Red
			color.RGBA{0, 255, 0, 255}, // Green
			color.RGBA{0, 0, 255, 255}, // Blue
		}
		err := createTestPNG(testImage, 100, 150, testColors)
		assert.NoError(t, err)

		processor := NewLogoProcessor(testImage, tempDir)
		err = processor.ExtractColors()
		assert.NoError(t, err)

		// Colors should have been updated (not default values)
		assert.NotEmpty(t, processor.Colors.Primary)
	})

	t.Run("extract colors from missing file", func(t *testing.T) {
		processor := NewLogoProcessor("/nonexistent/file.png", tempDir)
		err := processor.ExtractColors()
		assert.Error(t, err)
		// HXC-004 round-200 §11.4 (post-i18n): production emits message-ID
		// via NoopTranslator. See internal/logo/i18n/bundles/active.en.yaml.
		assert.Contains(t, err.Error(), "internal_logo_open_source_failed")
	})

	t.Run("extract colors from invalid image", func(t *testing.T) {
		// Create an invalid file (not an image)
		invalidFile := filepath.Join(tempDir, "invalid.png")
		err := os.WriteFile(invalidFile, []byte("not an image"), 0644)
		assert.NoError(t, err)

		processor := NewLogoProcessor(invalidFile, tempDir)
		err = processor.ExtractColors()
		assert.Error(t, err)
		// HXC-004 round-200 §11.4 (post-i18n): production emits message-ID
		// via NoopTranslator. See internal/logo/i18n/bundles/active.en.yaml.
		assert.Contains(t, err.Error(), "internal_logo_decode_source_failed")
	})
}

func TestGenerateASCIIArt(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logo_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("generate ASCII from valid image", func(t *testing.T) {
		// Create a simple test image
		testImage := filepath.Join(tempDir, "ascii_test.png")
		testColors := []color.Color{
			color.RGBA{0, 0, 0, 255},       // Black
			color.RGBA{128, 128, 128, 255}, // Gray
			color.RGBA{255, 255, 255, 255}, // White
		}
		err := createTestPNG(testImage, 80, 60, testColors)
		assert.NoError(t, err)

		processor := NewLogoProcessor(testImage, tempDir)
		ascii, err := processor.GenerateASCIIArt()
		assert.NoError(t, err)
		assert.NotEmpty(t, ascii)
		assert.Contains(t, ascii, "\n")
	})

	t.Run("generate ASCII from missing file", func(t *testing.T) {
		processor := NewLogoProcessor("/nonexistent/file.png", tempDir)
		_, err := processor.GenerateASCIIArt()
		assert.Error(t, err)
		// HXC-004 round-200 §11.4 (post-i18n): production emits message-ID
		// via NoopTranslator. See internal/logo/i18n/bundles/active.en.yaml.
		assert.Contains(t, err.Error(), "internal_logo_open_source_failed")
	})

	t.Run("generate ASCII from invalid image", func(t *testing.T) {
		invalidFile := filepath.Join(tempDir, "invalid_ascii.png")
		err := os.WriteFile(invalidFile, []byte("not an image"), 0644)
		assert.NoError(t, err)

		processor := NewLogoProcessor(invalidFile, tempDir)
		_, err = processor.GenerateASCIIArt()
		assert.Error(t, err)
		// HXC-004 round-200 §11.4 (post-i18n): production emits message-ID
		// via NoopTranslator. See internal/logo/i18n/bundles/active.en.yaml.
		assert.Contains(t, err.Error(), "internal_logo_decode_source_failed")
	})
}

func TestGenerateIcons(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logo_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("generate icons from valid image", func(t *testing.T) {
		// Create a test image large enough for resizing
		testImage := filepath.Join(tempDir, "icon_test.png")
		testColors := []color.Color{
			color.RGBA{100, 150, 200, 255},
		}
		err := createTestPNG(testImage, 512, 512, testColors)
		assert.NoError(t, err)

		// Create output directory structure
		iconsDir := filepath.Join(tempDir, "output", "icons")
		err = os.MkdirAll(iconsDir, 0755)
		assert.NoError(t, err)

		processor := NewLogoProcessor(testImage, filepath.Join(tempDir, "output"))
		err = processor.GenerateIcons()
		assert.NoError(t, err)

		// Verify some icons were created
		expectedIcons := []string{
			"icon-16x16.png",
			"icon-32x32.png",
			"icon-128x128.png",
			"icon-512x512.png",
		}
		for _, iconName := range expectedIcons {
			iconPath := filepath.Join(iconsDir, iconName)
			_, err := os.Stat(iconPath)
			assert.NoError(t, err, "Icon %s should exist", iconName)
		}
	})

	t.Run("generate icons from missing file", func(t *testing.T) {
		processor := NewLogoProcessor("/nonexistent/file.png", tempDir)
		err := processor.GenerateIcons()
		assert.Error(t, err)
	})

	t.Run("generate icons with missing output dir", func(t *testing.T) {
		testImage := filepath.Join(tempDir, "icon_test2.png")
		testColors := []color.Color{
			color.RGBA{100, 150, 200, 255},
		}
		err := createTestPNG(testImage, 100, 100, testColors)
		assert.NoError(t, err)

		processor := NewLogoProcessor(testImage, "/nonexistent/output")
		err = processor.GenerateIcons()
		assert.Error(t, err)
	})
}

func TestUpdateColorScheme(t *testing.T) {
	processor := NewLogoProcessor("test.png", "output")

	t.Run("update with two colors", func(t *testing.T) {
		colors := []color.Color{
			color.RGBA{255, 100, 50, 255},
			color.RGBA{50, 100, 255, 255},
		}
		processor.updateColorScheme(colors)
		assert.Equal(t, "#FF6432", processor.Colors.Primary)
		assert.Equal(t, "#3264FF", processor.Colors.Secondary)
	})

	t.Run("update with three colors", func(t *testing.T) {
		colors := []color.Color{
			color.RGBA{200, 50, 100, 255},
			color.RGBA{100, 200, 50, 255},
			color.RGBA{50, 100, 200, 255},
		}
		processor.updateColorScheme(colors)
		assert.Equal(t, "#C83264", processor.Colors.Primary)
		assert.Equal(t, "#64C832", processor.Colors.Secondary)
		assert.Equal(t, "#3264C8", processor.Colors.Accent)
	})

	t.Run("update with one color does nothing", func(t *testing.T) {
		originalPrimary := processor.Colors.Primary
		originalSecondary := processor.Colors.Secondary
		colors := []color.Color{
			color.RGBA{10, 20, 30, 255},
		}
		processor.updateColorScheme(colors)
		// Should not update with only one color
		assert.Equal(t, originalPrimary, processor.Colors.Primary)
		assert.Equal(t, originalSecondary, processor.Colors.Secondary)
	})
}

func TestSaveColorSchemeError(t *testing.T) {
	t.Run("save to non-existent directory", func(t *testing.T) {
		processor := NewLogoProcessor("test.png", "/nonexistent/path")
		err := processor.SaveColorScheme()
		assert.Error(t, err)
	})
}

func TestGenerateThemeFilesError(t *testing.T) {
	t.Run("generate to non-existent colors directory", func(t *testing.T) {
		processor := NewLogoProcessor("test.png", "/nonexistent/path")
		err := processor.GenerateThemeFiles()
		assert.Error(t, err)
	})
}
