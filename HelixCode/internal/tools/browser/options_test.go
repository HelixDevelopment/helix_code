package browser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOptionsFromEnv_Default_Headless(t *testing.T) {
	t.Setenv(EnvVarHeadedMode, "")
	o := OptionsFromEnv()
	require.True(t, o.Headless)
}

func TestOptionsFromEnv_TrueLowercase_Headed(t *testing.T) {
	t.Setenv(EnvVarHeadedMode, "true")
	o := OptionsFromEnv()
	require.False(t, o.Headless)
}

func TestOptionsFromEnv_TrueUppercase_Headed(t *testing.T) {
	t.Setenv(EnvVarHeadedMode, "TRUE")
	o := OptionsFromEnv()
	require.False(t, o.Headless)
}

func TestOptionsFromEnv_TrueMixedCase_Headed(t *testing.T) {
	t.Setenv(EnvVarHeadedMode, "True")
	o := OptionsFromEnv()
	require.False(t, o.Headless)
}

func TestOptionsFromEnv_NonBool_Headless(t *testing.T) {
	for _, v := range []string{"yes", "1", "True123", "headless", "0", "trueish", "TRUEZ"} {
		t.Setenv(EnvVarHeadedMode, v)
		o := OptionsFromEnv()
		require.True(t, o.Headless, "value %q should be headless", v)
	}
}

func TestOptionsFromEnv_DefaultViewport(t *testing.T) {
	t.Setenv(EnvVarHeadedMode, "")
	o := OptionsFromEnv()
	require.Equal(t, 1280, o.ViewportWidth)
	require.Equal(t, 720, o.ViewportHeight)
	require.Equal(t, 30*time.Second, o.NavigateTimeout)
	require.Equal(t, 500*time.Millisecond, o.ClickWaitDuration)
	require.Equal(t, MaxScreenshotBytes, o.ScreenshotMaxBytes)
}
