package com.example;

import com.vaadin.flow.component.html.Div;
import com.vaadin.flow.component.orderedlayout.VerticalLayout;
import com.vaadin.flow.router.Route;

/** Correct Java inline styling, plus the cases that must stay unreported. */
@Route("settings")
public class SettingsView extends VerticalLayout {

    public SettingsView() {
        Div box = new Div();

        // Correct: customize the inputs Aura computes from, and give a length a unit.
        box.getStyle().set("--aura-background-color-light", "khaki");
        box.getStyle().set("--aura-background-color-dark", "darkslateblue");
        box.getStyle().set("--aura-app-layout-inset", "0px");

        // Correct, and out of reach of the two selector checks: the class name and
        // the property are separate statements, so the Java pass does not judge
        // where the property landed.
        box.addClassNames("aura-surface", "recessed-box");
        box.getStyle().set("--aura-surface-level", "-1");
        box.getStyle().set("--aura-accent-color-light", "var(--aura-purple)");

        // A concatenation is not a literal: the value is not inspected, so the
        // unitless check stays quiet rather than reading the "0" as the value.
        String unit = "px";
        box.getStyle().set("--aura-app-layout-radius", "0" + unit);

        // Removing a property is not an assignment.
        box.getStyle().remove("--aura-background-color");

        // A raw style attribute is not parsed, so this read-only assignment goes
        // unreported. A known gap, kept here so the decision stays visible.
        box.getElement().setAttribute("style", "--aura-background-color: #fff");

        // Commented-out code is not code.
        // box.getStyle().set("--aura-red-text", "#900");

        add(box);
    }
}
