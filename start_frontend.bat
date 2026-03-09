@echo off
echo ====================================
echo  WoW AH Bot - Frontend (Angular)
echo ====================================
echo.
echo Demarrage du dashboard sur http://localhost:4200
echo.

cd /d "%~dp0frontend"
call npx ng serve --open

pause
