@echo off
rem Start a Spectrum Next on Windows with ZenZX's three example programs (gallop, fuji and
rem orbit) on the card in its second slot.
rem
rem     examples\next\run.cmd            (or double-click it)
rem
rem The first time, ZenZX opens a window that offers to download the Next's firmware (about
rem 70 MB). In NextZXOS press D in the Browser to change drive, then ENTER on a .nex file.
rem It needs zenzx.exe and Python, and offers to download either when it is not found.
setlocal
call "%~dp0..\tools\findpy.cmd"
if not defined PY ( pause & exit /b 1 )
%PY% "%~dp0..\launch.py" next %*
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" pause
exit /b %RC%
