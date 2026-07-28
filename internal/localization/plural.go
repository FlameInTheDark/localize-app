package localization

import (
	"fmt"
	"strings"
)

// PluralProfile contains the CLDR-style categories used by i18next and the
// matching GNU gettext rule written into PO headers.
type PluralProfile struct {
	Categories   []string
	POCategories []string
	PORule       string
}

func TargetPluralForms(language string) ([]string, error) {
	profile, err := PluralProfileFor(language)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), profile.Categories...), nil
}

func PluralProfileFor(language string) (PluralProfile, error) {
	if profile, ok := pluralProfiles[strings.ToLower(strings.TrimSpace(language))]; ok {
		return profile, nil
	}
	return PluralProfile{}, fmt.Errorf("plural forms require a built-in target language; %q is not supported", language)
}

var pluralProfiles = map[string]PluralProfile{
	"ar": {[]string{"zero", "one", "two", "few", "many", "other"}, []string{"zero", "one", "two", "few", "many", "other"}, "nplurals=6; plural=n==0 ? 0 : n==1 ? 1 : n==2 ? 2 : n%100>=3 && n%100<=10 ? 3 : n%100>=11 && n%100<=99 ? 4 : 5;"},
	"cs": {[]string{"one", "few", "other"}, []string{"one", "few", "other"}, "nplurals=3; plural=(n==1) ? 0 : (n>=2 && n<=4) ? 1 : 2;"},
	"he": {[]string{"one", "two", "many", "other"}, []string{"one", "two", "many", "other"}, "nplurals=4; plural=n==1 ? 0 : n==2 ? 1 : n%10==0 && n>10 ? 2 : 3;"},
	"pl": {[]string{"one", "few", "many", "other"}, []string{"one", "few", "many"}, "nplurals=3; plural=n==1 ? 0 : n%10>=2 && n%10<=4 && (n%100<10 || n%100>=20) ? 1 : 2;"},
	"ro": {[]string{"one", "few", "other"}, []string{"one", "few", "other"}, "nplurals=3; plural=n==1 ? 0 : (n==0 || (n%100>0 && n%100<20)) ? 1 : 2;"},
	"ru": {[]string{"one", "few", "many", "other"}, []string{"one", "few", "many"}, "nplurals=3; plural=n%10==1 && n%100!=11 ? 0 : n%10>=2 && n%10<=4 && (n%100<10 || n%100>=20) ? 1 : 2;"},
	"uk": {[]string{"one", "few", "many", "other"}, []string{"one", "few", "many"}, "nplurals=3; plural=n%10==1 && n%100!=11 ? 0 : n%10>=2 && n%10<=4 && (n%100<10 || n%100>=20) ? 1 : 2;"},
	"fr": {[]string{"one", "many", "other"}, []string{"one", "other"}, "nplurals=2; plural=n > 1;"},
	"ja": {[]string{"other"}, []string{"other"}, "nplurals=1; plural=0;"}, "ko": {[]string{"other"}, []string{"other"}, "nplurals=1; plural=0;"}, "zh": {[]string{"other"}, []string{"other"}, "nplurals=1; plural=0;"}, "vi": {[]string{"other"}, []string{"other"}, "nplurals=1; plural=0;"}, "th": {[]string{"other"}, []string{"other"}, "nplurals=1; plural=0;"}, "id": {[]string{"other"}, []string{"other"}, "nplurals=1; plural=0;"}, "tr": {[]string{"other"}, []string{"other"}, "nplurals=1; plural=0;"}, "fa": {[]string{"one", "other"}, []string{"one", "other"}, "nplurals=2; plural=n > 1;"},
}

func init() {
	standard := PluralProfile{Categories: []string{"one", "other"}, POCategories: []string{"one", "other"}, PORule: "nplurals=2; plural=(n != 1);"}
	for _, language := range []string{"en", "es", "de", "it", "pt", "hi", "bn", "nl", "sv", "no", "da", "fi", "el", "sw"} {
		pluralProfiles[language] = standard
	}
}
