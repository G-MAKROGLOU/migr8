@echo off
setlocal

REM Define the source and destination directories
set sourceAuthConfig=.\auth-config
set destAuthConfig=..\auth-config
set sourceCache=.\cache
set destCache=..\cache

REM Run robocopy commands
robocopy %sourceAuthConfig% %destAuthConfig% /E
robocopy %sourceCache% %destCache% /E

REM Remove the source directories
rmdir %sourceAuthConfig% /S /Q
rmdir %sourceCache% /S /Q

endlocal
