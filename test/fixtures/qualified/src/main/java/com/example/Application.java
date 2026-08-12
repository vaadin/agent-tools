package com.example;

import com.vaadin.flow.component.page.AppShellConfigurator;

// Both base themes loaded via fully qualified annotations (no imports) — the
// checker must detect these just as it does the unqualified @StyleSheet form.
@com.vaadin.flow.server.StyleSheet(com.vaadin.flow.theme.aura.Aura.STYLESHEET)
@com.vaadin.flow.server.StyleSheet(com.vaadin.flow.theme.lumo.Lumo.STYLESHEET)
public class Application implements AppShellConfigurator {
}
