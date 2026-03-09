@echo off
echo ====================================
echo  WoW AH Bot - Backend (Go)
echo ====================================
echo.
echo Demarrage du serveur API sur http://localhost:8000
echo.

cd /d "%~dp0backend-go"

REM Build if binary doesn't exist or source is newer
if not exist wow-ah-bot.exe (
    echo [INFO] Compilation du backend Go...
    go build -o wow-ah-bot.exe ./cmd/server/
    if errorlevel 1 (
        echo [ERREUR] La compilation a echoue. Verifiez que Go est installe.
        pause
        exit /b 1
    )
    echo [OK] Compilation reussie.
echo.
)

wow-ah-bot.exe

pause
