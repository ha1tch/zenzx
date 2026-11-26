@echo off
rem Launch the chat-room example on Windows: four Spectrums in one room, and a host that relays
rem each line to the others. Type a line in this window and press Enter to send it to all of
rem them; Ctrl-Z then Enter to leave.
rem
rem     examples\link\chat.cmd           (or double-click it)
rem
rem It needs zenzx.exe and Python, and offers to download either when it is not found
rem (see run.cmd in examples\world). launch.py does what chat.sh does on Linux and macOS.
setlocal
call "%~dp0..\tools\findpy.cmd"
if not defined PY ( pause & exit /b 1 )
%PY% "%~dp0..\launch.py" chat %*
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" pause
exit /b %RC%
