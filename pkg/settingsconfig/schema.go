package settingsconfig

import (
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

// MaxDisplayScale mirrors the display package's own MaxMultiplier --
// duplicated as a constant here rather than imported, since this
// package deliberately has no dependency on the GUI display layer
// (see the package doc comment for Theme/Font's own reasoning). Kept
// in sync by the two call sites that matter both being covered by
// tests: TestSchemaMaxDisplayScaleMatchesDisplayPackage in this
// package, and the display package's own MaxMultiplier tests.
const MaxDisplayScale = 5

// Schema builds the schema for settings.json, given the closed sets
// of valid theme and font names a "theme"/"font" field's own value
// must be one of.
func Schema(validThemes, validFonts []string) qf.Schema {
	return builders.Object().
		Field("version", builders.Number().Integer().Min(1).Required()).
		Field("theme", builders.String().Enum(validThemes...).Required()).
		Field("font", builders.String().Enum(validFonts...).Required()).
		Field("fontZoom", builders.Number().Integer().Min(1).Max(3).Required()).
		Field("displayScale", builders.Number().Integer().Min(1).Max(MaxDisplayScale).Required()).
		Field("fixedMenuBar", builders.Bool().Required())
}
