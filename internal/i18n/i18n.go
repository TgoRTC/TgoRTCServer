package i18n

// SupportedLanguages 支持的语言
var SupportedLanguages = []string{"zh-CN", "zh-TW", "en-US", "fr-FR", "ja-JP"}

// DefaultLanguage 默认语言
const DefaultLanguage = "zh-CN"

// LanguageContextKey 语言上下文键
const LanguageContextKey = "language"

// IsLanguageSupported 检查语言是否被支持
func IsLanguageSupported(lang string) bool {
	for _, supported := range SupportedLanguages {
		if supported == lang {
			return true
		}
	}
	return false
}

// GetLanguage 获取语言，如果不支持则返回默认语言
func GetLanguage(lang string) string {
	if IsLanguageSupported(lang) {
		return lang
	}
	return DefaultLanguage
}

