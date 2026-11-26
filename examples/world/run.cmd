@echo off
rem Launch the connected-world example on Windows: nine Spectrums in a grid, one world.
rem
rem     examples\world\run.cmd           (or double-click it)
rem
rem It needs zenzx.exe, which it looks for beside the examples folder, in a bin folder there,
rem or on the PATH, and offers to download when it is not found; and Python, which it offers to
rem download when there is none. launch.py does the work: it starts zenzx with the parameters
rem this example needs and then the host that ties the nine machines into one world, as run.sh does
rem on Linux and macOS. Extra arguments are given to launch.py: run.cmd --yes
setlocal
call "%~dp0..\tools\findpy.cmd"
if not defined PY ( pause & exit /b 1 )
%PY% "%~dp0..\launch.py" world %*
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" pause
exit /b %RC%
