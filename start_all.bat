@echo off
echo ====================================
echo  WoW AH Bot - Demarrage complet
echo ====================================
echo.

REM Verify .env exists
if not exist .env (
    echo [ERREUR] Le fichier .env n'existe pas.
    echo Copiez .env.example en .env et remplissez vos credentials.
    pause
    exit /b 1
)

echo Demarrage du Backend Go...
start "WoW AH Bot - Backend" cmd /c start_backend.bat

echo Attente de 3 secondes...
timeout /t 3 /nobreak >nul

echo Demarrage du Frontend...
start "WoW AH Bot - Frontend" cmd /c start_frontend.bat

echo.
echo ====================================
echo  Tout est lance !
echo ====================================
echo.
echo  Backend   : http://localhost:8000/api/dashboard
echo  Dashboard : http://localhost:4200
echo.
echo  Fermez cette fenetre quand vous voulez.
pause
