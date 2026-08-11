package com.example;

import com.vaadin.flow.component.page.AppShellConfigurator;
import com.vaadin.flow.server.StyleSheet;
import com.vaadin.flow.theme.lumo.Lumo;
import com.vaadin.flow.theme.lumo.LumoUtility;

@StyleSheet(Lumo.STYLESHEET)
public class Application implements AppShellConfigurator {
    static final String PAD = LumoUtility.Padding.MEDIUM;
}
