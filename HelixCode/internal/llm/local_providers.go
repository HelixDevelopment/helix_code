package llm

// GetLocalInferenceProviders returns the list of known local inference backends.
// These are software packages that can run models on local hardware.
// The list is maintained in one place (here) to avoid hardcoding in multiple
// command files (CONST-037 compliance).
func GetLocalInferenceProviders() []string {
	return []string{
		string(ProviderTypeVLLM),
		string(ProviderTypeLlamaCpp),
		string(ProviderTypeOllama),
		string(ProviderTypeLocalAI),
		string(ProviderTypeFastChat),
		"textgen",    // legacy name
		"lmstudio",   // GUI wrapper
		"jan",        // desktop app
		"koboldai",   // storytelling focused
		"gpt4all",    // consumer friendly
		"tabbyapi",   // exllama wrapper
		"mlx",        // Apple Silicon
		"mistralrs",  // Rust implementation
	}
}
