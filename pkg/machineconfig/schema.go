package machineconfig

import (
	qf "github.com/ha1tch/queryfy"
	"github.com/ha1tch/queryfy/builders"
)

// nodeSchema builds the schema for one Node. validModelIDs is the
// closed set a "model" node's own "id" must be one of. allowSubmenu
// controls whether a "submenu" shape is accepted at this level --
// true at the top level, false when building the schema for a
// submenu's own nested items, so a second level of nesting fails
// validation with a clear error rather than silently misbehaving at
// render time (see the Submenu doc comment in machineconfig.go for
// why one level is the limit).
func nodeSchema(validModelIDs []string, allowSubmenu bool) qf.Schema {
	separatorSchema := builders.Object().
		Field("type", builders.String().Enum(string(Separator)).Required())

	titleSchema := builders.Object().
		Field("type", builders.String().Enum(string(Title)).Required()).
		Field("label", builders.String().MinLength(1).Required()).
		Field("indent", builders.Number().Min(0).Integer())

	modelSchema := builders.Object().
		Field("type", builders.String().Enum(string(Model)).Required()).
		Field("id", builders.String().Enum(validModelIDs...).Required()).
		Field("label", builders.String().MinLength(1).Required()).
		Field("indent", builders.Number().Min(0).Integer())

	alternatives := []qf.Schema{separatorSchema, titleSchema, modelSchema}

	if allowSubmenu {
		submenuSchema := builders.Object().
			Field("type", builders.String().Enum(string(Submenu)).Required()).
			Field("label", builders.String().MinLength(1).Required()).
			Field("items", builders.Array().
				Of(nodeSchema(validModelIDs, false)).
				MinItems(1).
				Required())
		alternatives = append(alternatives, submenuSchema)
	}

	return builders.Or(alternatives...)
}

// ConfigSchema builds the schema for the whole machines.json document,
// given the closed set of valid -model identifiers a "model" node's
// own "id" may reference.
func ConfigSchema(validModelIDs []string) qf.Schema {
	return builders.Object().
		Field("version", builders.Number().Integer().Min(1).Required()).
		Field("items", builders.Array().
			Of(nodeSchema(validModelIDs, true)).
			MinItems(1).
			Required())
}
