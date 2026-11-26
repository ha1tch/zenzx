@echo off
rem findpy.cmd -- sets PY to a command that runs Python 3.6 or later, or says how to get one.
rem
rem     call "%~dp0..\tools\findpy.cmd"
rem     if not defined PY exit /b 1
rem     %PY% script.py
rem
rem Looked for, in order: the private copy (or environment) in examples\python, the "py"
rem launcher, "python" and "python3". A command counts only when it runs: the stub that
rem Windows keeps under the name "python" until Python is installed does not.
rem When there is none the person is told, in plain words, what Python is for, and Enter
rem has ZenZX fetch a private copy ("zenzx -fetch python -fetch-how private": python.org's
rem embeddable Python, which installs nothing; Windows 7 gets 3.8, the last that runs there).
rem The download is ZenZX's own, so it needs neither PowerShell, curl nor a TLS setting of
rem Windows 7's. ZENZX names the program; otherwise it is looked for where launch.py looks.
rem ZENZX_PYTHON_URL and ZENZX_PYTHON_SHA256 replace the download and its checksum, for the tests.
set "PY="
for %%I in ("%~dp0..") do set "EXAMPLES=%%~fI"
if exist "%EXAMPLES%\python\python.exe" call :try "%EXAMPLES%\python\python.exe"
if defined PY goto :eof
if exist "%EXAMPLES%\python\Scripts\python.exe" call :try "%EXAMPLES%\python\Scripts\python.exe"
if defined PY goto :eof
call :try py -3
if defined PY goto :eof
call :try python
if defined PY goto :eof
call :try python3
if defined PY goto :eof
goto :nopython

:try
%* -c "import sys; sys.exit(0 if sys.version_info >= (3, 6) else 1)" >nul 2>nul
if errorlevel 1 goto :eof
set PY=%*
goto :eof

:nopython
echo.
echo   These examples need a free helper program called Python.
echo   (The programming language, not the snake: a great many programs use it quietly.)
echo   It is not on this computer yet.
echo.
call :findzenzx
if not defined ZZ goto :nozenzx
echo   You do not need to install anything. If you press Enter, a private copy (about 10 MB,
echo   from python.org) goes into the folder "python" next to the examples. Nothing else on
echo   this computer changes, and deleting that folder removes it again.
echo.
echo   Or type O and press Enter to open python.org and install Python yourself,
echo   or type Q and press Enter to quit.
echo.
set "ANSWER=D"
set /p "ANSWER=  Press Enter to download it now: "
if /i "%ANSWER:~0,1%"=="Q" goto :eof
if /i "%ANSWER:~0,1%"=="N" goto :eof
if /i "%ANSWER:~0,1%"=="O" goto :openpage
start "" /wait /b "%ZZ%" -fetch python "-fetch-to=%EXAMPLES%\python" -fetch-how private "-fetch-url=%ZENZX_PYTHON_URL%" "-fetch-sha256=%ZENZX_PYTHON_SHA256%"
if errorlevel 1 goto :getfailed
call :try "%EXAMPLES%\python\python.exe"
if defined PY goto :eof
:getfailed
echo.
echo   Python could not be fetched (what went wrong is said above).
echo   You can get Python yourself from https://www.python.org/downloads/windows/
echo   (tick "Add python.exe to PATH" in its installer) and then run this again.
goto :eof

:findzenzx
set "ZZ="
if defined ZENZX if exist "%ZENZX%" set "ZZ=%ZENZX%"
if defined ZZ goto :eof
for %%N in (zenzx-headless.exe zenzx.exe) do for %%B in ("%EXAMPLES%\..\bin" "%EXAMPLES%\.." "%EXAMPLES%\..\dist" "%EXAMPLES%\..\..\bin" "%EXAMPLES%\..\..") do if not defined ZZ if exist "%%~B\%%N" set "ZZ=%%~fB\%%N"
if defined ZZ goto :eof
for %%N in (zenzx-headless.exe zenzx.exe) do if not defined ZZ for /f "delims=" %%P in ('where %%N 2^>nul') do if not defined ZZ set "ZZ=%%P"
goto :eof

:nozenzx
echo   The program ZenZX, which fetches it, is not where it can be found: put zenzx.exe in a
echo   folder called bin next to the folder of examples (https://ha1tch.github.io/zenzx/ has it)
echo   and run this again.
echo.
echo   Or install Python yourself from https://www.python.org/downloads/windows/
echo   (tick "Add python.exe to PATH" in its installer) and then run this again.
goto :eof

:openpage
start "" "https://www.python.org/downloads/windows/"
echo.
echo   Install Python (tick "Add python.exe to PATH" in its installer), then run this again.
goto :eof
