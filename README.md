# 🎮 WoW Auction House Trading Bot

Bot intelligent de trading pour l'Hôtel des Ventes de World of Warcraft (Retail).  
Serveur : **Archimonde (EU)**

## 🏗️ Architecture

```
├── backend/          # Python (FastAPI + SQLAlchemy + Discord.py)
│   ├── api/          # Endpoints REST pour le dashboard
│   ├── bot/          # Bot Discord
│   ├── core/         # Config, auth, database
│   ├── models/       # Modèles de données
│   ├── services/     # Logique métier (Blizzard API, trading engine)
│   └── scheduler/    # Tâches planifiées
├── frontend/         # Angular 17 + PrimeNG
│   └── src/
└── data/             # Base SQLite
```

## 🚀 Installation

### Prérequis
- Python 3.11+
- Node.js 18+
- Un compte Blizzard Developer (https://develop.battle.net/)
- Un bot Discord (https://discord.com/developers/)

### 1. Configuration
```bash
cp .env.example .env
# Remplir les valeurs dans .env
```

### 2. Backend
```bash
cd backend
pip install -r requirements.txt
python main.py
```

### 3. Frontend
```bash
cd frontend
npm install
ng serve
```

### 4. Accès
- **Dashboard** : http://localhost:4200
- **API** : http://localhost:8000/docs

## 📊 Fonctionnalités

- ✅ Récupération automatique des données de l'AH toutes les heures
- ✅ Algorithme de détection de bonnes affaires
- ✅ Scoring des opportunités d'investissement
- ✅ Bot Discord avec alertes d'achat/vente
- ✅ Dashboard avec graphiques P&L et historique des prix
- ✅ Suivi de portefeuille (achats, ventes, bénéfices)
