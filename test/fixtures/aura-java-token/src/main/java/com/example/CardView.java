package com.example;

import com.vaadin.flow.component.html.Div;
import com.vaadin.flow.component.orderedlayout.VerticalLayout;
import com.vaadin.flow.router.Route;

/** A project may define its own --aura-* token, and may define it from Java. */
@Route("cards")
public class CardView extends VerticalLayout {

    public CardView() {
        Div card = new Div();
        card.addClassName("card");
        card.getStyle().set("--aura-card-padding", "1rem");
        // Not a definition, because this line is commented out — the name is
        // written exactly as a definition would write it, quotes and all:
        // card.getStyle().set("--aura-card-margin", "0.5rem");
        add(card);
    }
}
