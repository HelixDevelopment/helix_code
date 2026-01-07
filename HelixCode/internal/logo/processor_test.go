package logo

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
