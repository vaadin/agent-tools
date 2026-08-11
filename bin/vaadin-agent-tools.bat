@echo off
rem Launcher for @vaadin/agent-tools (Windows).
rem
rem Runs the JS CLI without requiring a global Node.js install. Resolves a
rem usable node in this order:
rem   1. %VAADIN_AGENT_TOOLS_NODE%   (explicit override)
rem   2. node on PATH
rem   3. Node downloaded by Vaadin   (%USERPROFILE%\.vaadin\node)
rem   4. Project-local node\         (some Vaadin setups install here)
rem
rem Exit codes mirror the CLI (0 ok, 1 findings, 2 usage) plus 3 = no runtime.
setlocal
set "BIN_DIR=%~dp0"
set "CLI=%BIN_DIR%..\src\cli.js"

set "NODE="
if defined VAADIN_AGENT_TOOLS_NODE if exist "%VAADIN_AGENT_TOOLS_NODE%" set "NODE=%VAADIN_AGENT_TOOLS_NODE%"
if not defined NODE where node >nul 2>nul && set "NODE=node"
if not defined NODE if exist "%USERPROFILE%\.vaadin\node\node.exe" set "NODE=%USERPROFILE%\.vaadin\node\node.exe"
if not defined NODE if exist "%USERPROFILE%\.vaadin\node\bin\node.exe" set "NODE=%USERPROFILE%\.vaadin\node\bin\node.exe"
if not defined NODE if exist "%CD%\node\node.exe" set "NODE=%CD%\node\node.exe"

if not defined NODE (
  echo vaadin-agent-tools: could not find a Node.js runtime.>&2
  echo Install Node.js 18.3+, run `mvnw vaadin:prepare-frontend` to let Vaadin>&2
  echo download Node under %%USERPROFILE%%\.vaadin\node, or set VAADIN_AGENT_TOOLS_NODE.>&2
  exit /b 3
)

"%NODE%" "%CLI%" %*
