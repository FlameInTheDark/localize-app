export type Language = { code: string; name: string; native?: string };

export const languages: Language[] = [
  { code: "en", name: "English" }, { code: "es", name: "Spanish", native: "Español" }, { code: "fr", name: "French", native: "Français" }, { code: "de", name: "German", native: "Deutsch" },
  { code: "it", name: "Italian", native: "Italiano" }, { code: "pt", name: "Portuguese", native: "Português" }, { code: "ru", name: "Russian", native: "Русский" }, { code: "uk", name: "Ukrainian", native: "Українська" },
  { code: "pl", name: "Polish", native: "Polski" }, { code: "tr", name: "Turkish", native: "Türkçe" }, { code: "ar", name: "Arabic", native: "العربية" }, { code: "hi", name: "Hindi", native: "हिन्दी" },
  { code: "bn", name: "Bengali", native: "বাংলা" }, { code: "zh", name: "Chinese", native: "中文" }, { code: "ja", name: "Japanese", native: "日本語" }, { code: "ko", name: "Korean", native: "한국어" },
  { code: "vi", name: "Vietnamese", native: "Tiếng Việt" }, { code: "th", name: "Thai", native: "ไทย" }, { code: "id", name: "Indonesian", native: "Bahasa Indonesia" }, { code: "nl", name: "Dutch", native: "Nederlands" },
  { code: "sv", name: "Swedish", native: "Svenska" }, { code: "no", name: "Norwegian", native: "Norsk" }, { code: "da", name: "Danish", native: "Dansk" }, { code: "fi", name: "Finnish", native: "Suomi" },
  { code: "cs", name: "Czech", native: "Čeština" }, { code: "ro", name: "Romanian", native: "Română" }, { code: "el", name: "Greek", native: "Ελληνικά" }, { code: "he", name: "Hebrew", native: "עברית" },
  { code: "fa", name: "Persian", native: "فارسی" }, { code: "sw", name: "Swahili", native: "Kiswahili" },
];
