package com.example;

import com.vaadin.flow.component.html.Div;
import com.vaadin.flow.component.orderedlayout.VerticalLayout;
import com.vaadin.flow.router.Route;
import com.vaadin.flow.signals.local.ValueSignal;

/** An inline style set from Java breaks the same Aura rules as a stylesheet. */
@Route("dashboard")
public class DashboardView extends VerticalLayout {

    public DashboardView(String accent, String radius, ValueSignal<String> surface,
            ThemeConfig config) {
        Div box = new Div();

        // Read-only: Aura computes it with light-dark(), so this breaks dark mode.
        box.getStyle().set("--aura-background-color", "#fff");

        // Read-only while Aura is the loaded theme, which reassigns it.
        box.getElement().getStyle().set("--vaadin-text-color", "#222");

        // A length fed into calc(): a unitless zero invalidates the expression.
        box.getStyle().set("--aura-app-layout-inset", "0");

        // The value is not a literal, so it cannot be read here — but the property
        // name still decides the read-only check, and both of these are read-only.
        box.getStyle().set("--aura-accent-color", accent);
        box.getStyle().bind("--aura-surface-color", surface);

        // Same, on a length property: nothing to inspect, so no unitless finding.
        box.getStyle().set("--aura-app-layout-radius", radius);

        // Not a Style at all. Knowingly reported: the call has the same shape, and
        // requiring the -- prefix keeps such a line worth a look either way.
        config.set("--aura-font-size-m", "15px");

        add(box);
    }

    /** Stands in for any non-Style API with a set(String, String) of its own. */
    interface ThemeConfig {
        void set(String name, String value);
    }
}
