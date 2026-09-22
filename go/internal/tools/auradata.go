package tools

// Aura theme reference data for check-aura-usage.
//
// The property names are measured from @vaadin/aura (see
// scripts/derive-aura-properties.sh, which re-derives them and diffs against the
// tables below so drift shows up per release).
//
// The write-safety classification is NOT derivable from the CSS: several
// properties are computed from other properties and are still the documented,
// intended override points. The split below follows the badges in the Vaadin
// Aura reference pages — a property carrying `Read-only` or `light-dark()` is
// read-only, everything else is customizable.
//
// Measured from @vaadin/aura@25.3.0-rc1: 74 defined properties + 3 that Aura
// only reads through a fallback (customization hooks) = 77 known names.

// auraKnownProperties is every --aura-* custom property the theme defines or
// reads. A var() read of a name outside this set — and not defined anywhere in
// the project — is a typo or a hallucinated token.
var auraKnownProperties = map[string]bool{
	"--aura-accent-border-color":         true,
	"--aura-accent-color":                true,
	"--aura-accent-color-dark":           true,
	"--aura-accent-color-dark-initial":   true,
	"--aura-accent-color-light":          true,
	"--aura-accent-color-light-initial":  true,
	"--aura-accent-contrast-color":       true,
	"--aura-accent-contrast-color-dark":  true,
	"--aura-accent-contrast-color-light": true,
	"--aura-accent-surface":              true,
	"--aura-accent-text-color":           true,
	"--aura-accent-text-color-dark":      true,
	"--aura-accent-text-color-light":     true,
	"--aura-app-background":              true,
	"--aura-app-layout-border-width":     true,
	"--aura-app-layout-inset":            true,
	"--aura-app-layout-radius":           true,
	"--aura-background-color":            true,
	"--aura-background-color-dark":       true,
	"--aura-background-color-light":      true,
	"--aura-base-font-size":              true,
	"--aura-base-line-height":            true,
	"--aura-base-radius":                 true,
	"--aura-base-size":                   true,
	"--aura-blue":                        true,
	"--aura-blue-text":                   true,
	"--aura-color-scheme":                true,
	"--aura-content-color-scheme":        true,
	"--aura-contrast-level":              true,
	"--aura-font-family":                 true,
	"--aura-font-family-instrument-sans": true,
	"--aura-font-family-system":          true,
	"--aura-font-size-l":                 true,
	"--aura-font-size-m":                 true,
	"--aura-font-size-s":                 true,
	"--aura-font-size-xl":                true,
	"--aura-font-size-xs":                true,
	"--aura-font-smoothing":              true,
	"--aura-font-weight-medium":          true,
	"--aura-font-weight-regular":         true,
	"--aura-font-weight-semibold":        true,
	"--aura-green":                       true,
	"--aura-green-text":                  true,
	"--aura-item-overlay-padding-block":  true,
	"--aura-item-overlay-padding-inline": true,
	"--aura-line-height-l":               true,
	"--aura-line-height-m":               true,
	"--aura-line-height-s":               true,
	"--aura-line-height-xl":              true,
	"--aura-line-height-xs":              true,

	"--aura-master-detail-layout-detail-inset": true,

	"--aura-neutral":                     true,
	"--aura-neutral-dark":                true,
	"--aura-neutral-light":               true,
	"--aura-notification-color-scheme":   true,
	"--aura-orange":                      true,
	"--aura-orange-text":                 true,
	"--aura-overlay-backdrop-filter":     true,
	"--aura-overlay-inner-outline-color": true,
	"--aura-overlay-outline-color":       true,
	"--aura-overlay-outline-shadow":      true,
	"--aura-overlay-shadow":              true,
	"--aura-overlay-surface-opacity":     true,
	"--aura-purple":                      true,
	"--aura-purple-text":                 true,
	"--aura-red":                         true,
	"--aura-red-text":                    true,
	"--aura-shadow-color":                true,
	"--aura-shadow-m":                    true,
	"--aura-shadow-s":                    true,
	"--aura-shadow-xs":                   true,
	"--aura-surface-color":               true,
	"--aura-surface-color-solid":         true,
	"--aura-surface-level":               true,
	"--aura-surface-opacity":             true,
	"--aura-yellow":                      true,
	"--aura-yellow-text":                 true,
}

// auraReadOnly describes a property Aura computes and that must not be assigned:
// reason explains what the assignment breaks, instead names the customizable
// property to set in its place, and level is the finding level.
type auraReadOnly struct {
	level   string
	reason  string
	instead string
}

// lightDarkReason is shared by every property Aura defines through the CSS
// light-dark() function: assigning one value replaces the whole light-dark()
// pair, so the property stops adapting to the active color scheme.
const lightDarkReason = "Aura computes it with light-dark(), so assigning a single value " +
	"discards both the light and the dark color-scheme value."

// auraReadOnlyProperties are the 27 --aura-* properties documented as read-only.
// They are never safe to assign; the customizable input is named in instead.
var auraReadOnlyProperties = map[string]auraReadOnly{
	"--aura-accent-border-color": {"error", lightDarkReason,
		"--aura-contrast-level, or the accent color via --aura-accent-color-light / --aura-accent-color-dark"},
	"--aura-accent-color": {"error", lightDarkReason,
		"--aura-accent-color-light / --aura-accent-color-dark"},
	"--aura-accent-contrast-color": {"error", lightDarkReason,
		"--aura-accent-contrast-color-light / --aura-accent-contrast-color-dark"},
	"--aura-accent-surface": {"error",
		"Aura computes it by blending the effective accent color into the surface color.",
		"the accent color (--aura-accent-color-light / --aura-accent-color-dark, or an aura-accent-* class name) " +
			"together with --aura-surface-level / --aura-surface-opacity"},
	"--aura-accent-text-color": {"error", lightDarkReason,
		"--aura-accent-text-color-light / --aura-accent-text-color-dark"},
	"--aura-background-color": {"error", lightDarkReason,
		"--aura-background-color-light / --aura-background-color-dark"},
	"--aura-blue-text":   {"error", lightDarkReason, "--aura-blue"},
	"--aura-green-text":  {"error", lightDarkReason, "--aura-green"},
	"--aura-orange-text": {"error", lightDarkReason, "--aura-orange"},
	"--aura-purple-text": {"error", lightDarkReason, "--aura-purple"},
	"--aura-red-text":    {"error", lightDarkReason, "--aura-red"},
	"--aura-yellow-text": {"error", lightDarkReason, "--aura-yellow"},
	"--aura-neutral": {"error", lightDarkReason,
		"--aura-neutral-light / --aura-neutral-dark"},

	"--aura-font-size-xs": {"error", auraFontSizeReason, "--aura-base-font-size"},
	"--aura-font-size-s":  {"error", auraFontSizeReason, "--aura-base-font-size"},
	"--aura-font-size-m":  {"error", auraFontSizeReason, "--aura-base-font-size"},
	"--aura-font-size-l":  {"error", auraFontSizeReason, "--aura-base-font-size"},
	"--aura-font-size-xl": {"error", auraFontSizeReason, "--aura-base-font-size"},

	"--aura-line-height-xs": {"error", auraLineHeightReason, "--aura-base-line-height"},
	"--aura-line-height-s":  {"error", auraLineHeightReason, "--aura-base-line-height"},
	"--aura-line-height-m":  {"error", auraLineHeightReason, "--aura-base-line-height"},
	"--aura-line-height-l":  {"error", auraLineHeightReason, "--aura-base-line-height"},
	"--aura-line-height-xl": {"error", auraLineHeightReason, "--aura-base-line-height"},

	"--aura-surface-color": {"error",
		"Aura computes it from --aura-background-color, --aura-surface-level and --aura-surface-opacity.",
		"--aura-surface-level / --aura-surface-opacity"},
	"--aura-surface-color-solid": {"error",
		"Aura computes it from --aura-background-color and --aura-surface-level.",
		"--aura-surface-level"},

	// Milder case: redefining a font stack is wrong usage rather than a
	// color-scheme bug, so these are warnings.
	"--aura-font-family-system": {"warning",
		"It is one of Aura's two font stacks, meant to be assigned to --aura-font-family rather than redefined.",
		"--aura-font-family"},
	"--aura-font-family-instrument-sans": {"warning",
		"It is one of Aura's two font stacks, meant to be assigned to --aura-font-family rather than redefined.",
		"--aura-font-family"},
}

const auraFontSizeReason = "Aura computes the font size scale from --aura-base-font-size."
const auraLineHeightReason = "Aura computes the line height scale from --aura-base-line-height and the font sizes."

// auraReassignedVaadinProperties are the 7 base-style --vaadin-* properties Aura
// reassigns with its own computed, color-scheme-adapted values. They are only
// read-only while Aura is the loaded theme, so these are gated on that.
var auraReassignedVaadinProperties = map[string]auraReadOnly{
	"--vaadin-background-container": {"error", lightDarkReason,
		"--aura-background-color-light / --aura-background-color-dark, or --aura-contrast-level"},
	"--vaadin-background-container-strong": {"error", lightDarkReason,
		"--aura-background-color-light / --aura-background-color-dark, or --aura-contrast-level"},
	"--vaadin-border-color": {"error", lightDarkReason,
		"--aura-contrast-level, or --aura-background-color-light / --aura-background-color-dark"},
	"--vaadin-border-color-secondary": {"error", lightDarkReason,
		"--aura-contrast-level, or --aura-background-color-light / --aura-background-color-dark"},
	"--vaadin-text-color": {"error", lightDarkReason,
		"--aura-neutral-light / --aura-neutral-dark"},
	"--vaadin-text-color-secondary": {"error",
		lightDarkReason + " It is derived from the main text color.",
		"--aura-contrast-level, or --aura-neutral-light / --aura-neutral-dark"},
	"--vaadin-text-color-disabled": {"error",
		lightDarkReason + " It is derived from the main text color.",
		"--aura-contrast-level, or --aura-neutral-light / --aura-neutral-dark"},
}

// auraLengthProperties are the Aura properties whose value is a CSS length and
// that feed a calc()/clamp()/min()/max() expression. A unitless number — most
// often a bare 0 — makes the whole expression invalid, so the declaration is
// dropped silently.
var auraLengthProperties = map[string]bool{
	"--aura-app-layout-inset":            true,
	"--aura-app-layout-radius":           true,
	"--aura-app-layout-border-width":     true,
	"--aura-item-overlay-padding-inline": true,
	"--aura-item-overlay-padding-block":  true,
}

// auraSurfaceProperties are the two properties that only take effect on an
// element that actually uses the Aura surface color.
var auraSurfaceProperties = []string{"--aura-surface-level", "--aura-surface-opacity"}

// auraAccentInputProperties are the accent color inputs that only take effect on
// an element the accent color is (re)computed on — i.e. one carrying an
// aura-accent-* class name. (--aura-accent-color itself is read-only and is
// reported by the read-only check instead.)
var auraAccentInputProperties = []string{"--aura-accent-color-light", "--aura-accent-color-dark"}

// auraSurfaceSelectors are the selectors Aura defines the surface color on, and
// therefore the ones that respond to --aura-surface-level / --aura-surface-opacity.
// Derived from the selector list in @vaadin/aura/src/surface.css, plus the table
// in the Aura color reference page.
var auraSurfaceSelectors = []string{
	":root", ":host", "html", "body",
	".aura-surface", ".aura-surface-solid",
	"::part(input-field)", "::part(overlay)",
	"vaadin-accordion-panel", "vaadin-app-layout", "vaadin-avatar-group", "vaadin-button",
	"vaadin-card", "vaadin-checkbox", "vaadin-combo-box", "vaadin-confirm-dialog",
	"vaadin-context-menu", "vaadin-crud", "vaadin-dashboard-widget", "vaadin-date-picker",
	"vaadin-details", "vaadin-dialog", "vaadin-drawer-toggle", "vaadin-grid", "vaadin-grid-pro",
	"vaadin-login-overlay", "vaadin-master-detail-layout", "vaadin-menu-bar-button",
	"vaadin-menu-bar-submenu", "vaadin-message", "vaadin-message-input",
	"vaadin-multi-select-combo-box", "vaadin-notification-card", "vaadin-popover",
	"vaadin-radio-button", "vaadin-range-slider", "vaadin-rich-text-editor",
	"vaadin-rich-text-editor-popup", "vaadin-select", "vaadin-side-nav-item", "vaadin-slider",
	"vaadin-slider-bubble", "vaadin-tab", "vaadin-tabs", "vaadin-time-picker", "vaadin-tooltip",
	"vaadin-upload", "vaadin-upload-button", "vaadin-upload-file",
}

// auraAccentSelectors are the selectors Aura recomputes the accent color and its
// derived colors (--aura-accent-surface, --aura-accent-text-color, …) on.
// Derived from the selector list in @vaadin/aura/src/color.css.
var auraAccentSelectors = []string{
	":root", ":host", "html", "body",
	".aura-accent-color", ".aura-accent-neutral", ".aura-accent-red", ".aura-accent-orange",
	".aura-accent-yellow", ".aura-accent-green", ".aura-accent-blue", ".aura-accent-purple",
	".aura-accent-surface", ".aura-accent-surface-solid",
	".v-info", ".v-success", ".v-warning", ".v-error",
	"[theme~='danger']", "[theme~='error']", "[theme~='success']", "[theme~='warning']",
	"[theme~='info']",
	"vaadin-app-layout", "vaadin-avatar", "vaadin-badge", "vaadin-button", "vaadin-checkbox",
	"vaadin-confirm-dialog", "vaadin-context-menu-item", "vaadin-crud-edit", "vaadin-dialog",
	"vaadin-drawer-toggle", "vaadin-menu-bar-button", "vaadin-menu-bar-item", "vaadin-message",
	"vaadin-notification-card", "vaadin-notification-container", "vaadin-radio-button",
	"vaadin-select-item", "vaadin-side-nav-item", "vaadin-tab", "vaadin-upload-button",
	"vaadin-upload-file",
}

// auraSurfaceRespondingSelectors are every selector on which
// --aura-surface-level / --aura-surface-opacity have an effect. That is the
// surface selectors plus the accent ones: Aura computes --aura-accent-surface
// from those same two properties, so an element it recomputes the accent colors
// on — a vaadin-badge, an .aura-accent-surface box — responds to them too.
var auraSurfaceRespondingSelectors = append(
	append([]string{}, auraSurfaceSelectors...), auraAccentSelectors...)

// auraAccentClassNames are the class names the docs tell you to apply when you
// want a different accent color on part of the UI; named in the finding message.
const auraAccentClassNames = "aura-accent-neutral, aura-accent-red, aura-accent-orange, " +
	"aura-accent-yellow, aura-accent-green, aura-accent-blue, aura-accent-purple"
