@echo off
rem Selector for the vaadin-agent-tools plugin on Windows. Execs the bundled
rem native binary shipped inside this plugin (bin\platform\). No Node or JVM.
"%~dp0platform\vaadin-agent-tools-windows-amd64.exe" %*
