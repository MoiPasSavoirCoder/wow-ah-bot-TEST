@echo off
echo ====================================
echo  WoW AH Trading Bot - Installation
echo ====================================
echo.

REM Check Go
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERREUR] Go n'est pas installe.
    echo Telechargez Go sur https://go.dev/dl/ ^(version 1.21+^)
    pause
    exit /b 1
)

REM Check Node
node --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERREUR] Node.js n'est pas installe. Installez Node.js 18+
    pause
    exit /b 1
)

echo [1/4] Creation du fichier .env...
if not exist .env (
    copy .env.example .env
    echo       .env cree ! Editez-le avec vos credentials.
) else (
    echo       .env existe deja.
)

echo.
echo [2/4] Creation du dossier data...
if not exist data mkdir data

echo.
echo [3/4] Telechargement des dependances Go et compilation...
cd backend-go
go mod tidy
go build -o wow-ah-bot.exe ./cmd/server/
if errorlevel 1 (
    echo [ERREUR] La compilation Go a echoue.
    pause
    exit /b 1
)
echo       Backend compile : backend-go\wow-ah-bot.exe
cd ..

echo.
echo [4/4] Installation des dependances Frontend...
cd frontend
call npm install
cd ..

echo.
echo ====================================
echo  Installation terminee !
echo ====================================
echo.
echo Prochaines etapes :
echo  1. Editez le fichier .env avec vos credentials Blizzard et Discord
echo  2. Lancez tout : start_all.bat
echo     OU separement :
echo       - Backend  : start_backend.bat
echo       - Frontend : start_frontend.bat
echo  3. Ouvrez http://localhost:4200
echo.
pause
