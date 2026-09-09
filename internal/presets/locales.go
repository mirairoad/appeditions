package presets

import "github.com/mirairoad/appeditions/internal/model"

// Locales are the App Store's localisation codes, which Google Play accepts a
// superset of. The tag is what the export directory is named after, so it has
// to be the store's spelling rather than a friendly one: `pt-BR`, not
// `portuguese-brazil`.
var Locales = []model.Locale{
	{Tag: "en-US", Label: "English (US)", Native: "English"},
	{Tag: "en-GB", Label: "English (UK)", Native: "English"},
	{Tag: "ja", Label: "Japanese", Native: "日本語"},
	{Tag: "ko", Label: "Korean", Native: "한국어"},
	{Tag: "zh-Hans", Label: "Chinese (Simplified)", Native: "简体中文"},
	{Tag: "zh-Hant", Label: "Chinese (Traditional)", Native: "繁體中文"},
	{Tag: "de-DE", Label: "German", Native: "Deutsch"},
	{Tag: "fr-FR", Label: "French", Native: "Français"},
	{Tag: "es-ES", Label: "Spanish (Spain)", Native: "Español"},
	{Tag: "es-MX", Label: "Spanish (Mexico)", Native: "Español"},
	{Tag: "pt-BR", Label: "Portuguese (Brazil)", Native: "Português"},
	{Tag: "pt-PT", Label: "Portuguese (Portugal)", Native: "Português"},
	{Tag: "it", Label: "Italian", Native: "Italiano"},
	{Tag: "nl-NL", Label: "Dutch", Native: "Nederlands"},
	{Tag: "sv", Label: "Swedish", Native: "Svenska"},
	{Tag: "da", Label: "Danish", Native: "Dansk"},
	{Tag: "no", Label: "Norwegian", Native: "Norsk"},
	{Tag: "fi", Label: "Finnish", Native: "Suomi"},
	{Tag: "pl", Label: "Polish", Native: "Polski"},
	{Tag: "ru", Label: "Russian", Native: "Русский"},
	{Tag: "tr", Label: "Turkish", Native: "Türkçe"},
	{Tag: "id", Label: "Indonesian", Native: "Bahasa Indonesia"},
	{Tag: "th", Label: "Thai", Native: "ไทย"},
	{Tag: "vi", Label: "Vietnamese", Native: "Tiếng Việt"},
	{Tag: "hi", Label: "Hindi", Native: "हिन्दी"},
	{Tag: "ar-SA", Label: "Arabic", Native: "العربية", RTL: true},
	{Tag: "he", Label: "Hebrew", Native: "עברית", RTL: true},
	{Tag: "uk", Label: "Ukrainian", Native: "Українська"},
	{Tag: "cs", Label: "Czech", Native: "Čeština"},
	{Tag: "el", Label: "Greek", Native: "Ελληνικά"},
	{Tag: "ro", Label: "Romanian", Native: "Română"},
	{Tag: "hu", Label: "Hungarian", Native: "Magyar"},
	{Tag: "ms", Label: "Malay", Native: "Bahasa Melayu"},
	{Tag: "ca", Label: "Catalan", Native: "Català"},
	{Tag: "hr", Label: "Croatian", Native: "Hrvatski"},
	{Tag: "sk", Label: "Slovak", Native: "Slovenčina"},
}

// LocaleOf resolves a tag. An unknown one comes back as itself rather than a
// fallback: a project stored with a locale this table has not heard of should
// keep shipping that language, not silently become English.
func LocaleOf(tag string) model.Locale {
	for _, l := range Locales {
		if l.Tag == tag {
			return l
		}
	}
	return model.Locale{Tag: tag, Label: tag}
}
