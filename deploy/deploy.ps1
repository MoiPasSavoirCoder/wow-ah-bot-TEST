# ============================================================
#  deploy.ps1 — Déploie WoW AH Bot sur le VPS Debian
#  Usage : .\deploy\deploy.ps1
#  Prérequis : Go, Node/Angular CLI installés en local
#              ssh configuré (clé SSH ou mot de passe)
# ============================================================

$SERVER      = "debian@57.128.246.255"
$REMOTE_APP  = "/opt/wow-ah-bot"
$REMOTE_WEB  = "/var/www/wow-ah-bot"
$BINARY_NAME = "wow-ah-bot"

Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  WoW AH Bot - Déploiement VPS" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan

# ── 1. Compilation Go pour Linux ─────────────────────────────
Write-Host "`n[1/5] Compilation du backend Go pour Linux (amd64)..." -ForegroundColor Yellow
$env:GOOS   = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

Push-Location "$PSScriptRoot\..\backend-go"
go build -o "..\deploy\$BINARY_NAME" .\cmd\server\main.go
if ($LASTEXITCODE -ne 0) { Write-Host "[ERREUR] Compilation Go échouée." -ForegroundColor Red; exit 1 }
Pop-Location

Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
Write-Host "  -> Binaire Linux créé : deploy\$BINARY_NAME" -ForegroundColor Green

# ── 2. Build Angular ─────────────────────────────────────────
Write-Host "`n[2/5] Build du frontend Angular..." -ForegroundColor Yellow
Push-Location "$PSScriptRoot\..\frontend"
ng build --configuration production
if ($LASTEXITCODE -ne 0) { Write-Host "[ERREUR] ng build échoué." -ForegroundColor Red; exit 1 }
Pop-Location
Write-Host "  -> Build Angular terminé : frontend\dist\" -ForegroundColor Green

# ── 3. Création des dossiers sur le serveur ──────────────────
Write-Host "`n[3/5] Préparation des dossiers sur le serveur..." -ForegroundColor Yellow
ssh $SERVER "sudo mkdir -p $REMOTE_APP $REMOTE_WEB && sudo chown -R debian:debian $REMOTE_APP $REMOTE_WEB"

# ── 4. Envoi des fichiers via SCP ────────────────────────────
Write-Host "`n[4/5] Envoi des fichiers sur le serveur..." -ForegroundColor Yellow

# Binaire Go
scp "$PSScriptRoot\$BINARY_NAME" "${SERVER}:${REMOTE_APP}/${BINARY_NAME}"

# Frontend Angular (cherche dans browser/ ou directement dans dist/)
$distPath = "$PSScriptRoot\..\frontend\dist"
$browserPath = Get-ChildItem "$distPath" -Recurse -Filter "index.html" | Select-Object -First 1
if ($browserPath) {
    $frontendDist = $browserPath.DirectoryName
} else {
    $frontendDist = $distPath
}
Write-Host "  -> Envoi frontend depuis : $frontendDist" -ForegroundColor Gray
scp -r "${frontendDist}\*" "${SERVER}:${REMOTE_WEB}/"

# Config nginx + service systemd
scp "$PSScriptRoot\nginx.conf"       "${SERVER}:/tmp/wow-ah-bot-nginx.conf"
scp "$PSScriptRoot\wow-ah-bot.service" "${SERVER}:/tmp/wow-ah-bot.service"

# ── 5. Configuration sur le serveur ─────────────────────────
Write-Host "`n[5/5] Configuration du serveur (nginx + systemd)..." -ForegroundColor Yellow
ssh $SERVER @"
  # Rend le binaire exécutable
  chmod +x $REMOTE_APP/$BINARY_NAME

  # Installe le service systemd
  sudo mv /tmp/wow-ah-bot.service /etc/systemd/system/wow-ah-bot.service
  sudo systemctl daemon-reload
  sudo systemctl enable wow-ah-bot
  sudo systemctl restart wow-ah-bot
  echo '  -> Service wow-ah-bot démarré'

  # Installe la config nginx
  sudo mv /tmp/wow-ah-bot-nginx.conf /etc/nginx/sites-available/wow-ah-bot
  sudo ln -sf /etc/nginx/sites-available/wow-ah-bot /etc/nginx/sites-enabled/wow-ah-bot
  sudo nginx -t && sudo systemctl reload nginx
  echo '  -> Nginx rechargé'
"@

Write-Host "`n======================================" -ForegroundColor Green
Write-Host "  Déploiement terminé !" -ForegroundColor Green
Write-Host "======================================" -ForegroundColor Green
Write-Host "  App disponible sur : http://57.128.246.255" -ForegroundColor Cyan
Write-Host "  Logs backend       : ssh $SERVER 'journalctl -u wow-ah-bot -f'" -ForegroundColor Gray
Write-Host ""
Write-Host "  RAPPEL : Si c'est le premier déploiement," -ForegroundColor Yellow
Write-Host "  copie ton fichier .env sur le serveur :" -ForegroundColor Yellow
Write-Host "  scp .env ${SERVER}:${REMOTE_APP}/.env" -ForegroundColor Yellow
Write-Host ""
