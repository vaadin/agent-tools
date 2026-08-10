package com.example;

import com.vaadin.flow.component.page.AppShellConfigurator;
import com.vaadin.flow.server.StyleSheet;
import com.vaadin.flow.theme.aura.Aura;
import com.vaadin.flow.theme.lumo.Lumo;

@StyleSheet(Aura.STYLESHEET)
@StyleSheet(Lumo.STYLESHEET)
@StyleSheet("styles.css")
public class Application implements AppShellConfigurator {
}
