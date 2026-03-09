# 📖 Comment fonctionne WoW AH Bot

> **WoW AH Bot** est un outil de trading automatisé pour l'Hôtel des Ventes de World of Warcraft (Archimonde EU — Retail).
> Il scanne l'AH, détecte des opportunités d'achat/revente rentables, vous notifie via Discord et vous fournit un dashboard complet pour suivre vos gains et pertes.

---

## 🧑‍💻 PARTIE UTILISATEUR

### Ce que fait l'application pour vous

| Fonctionnalité | Description |
|---|---|
| 🔍 **Scan de l'AH** | Récupère automatiquement toutes les enchères actives sur Archimonde EU toutes les 60 minutes |
| 📊 **Analyse des opportunités** | Compare les prix actuels aux prix historiques (7 jours) pour repérer les bonnes affaires |
| 🤖 **Alertes Discord** | Vous envoie un message privé ou dans un canal Discord quand un deal rentable est détecté |
| 💼 **Suivi de portfolio** | Enregistrez vos achats/ventes pour calculer votre profit net |
| 📈 **Historique de prix** | Visualisez l'évolution du prix de n'importe quel item sur 1 à 90 jours |

---

### 🚀 Démarrer l'application

**1. Lancer le backend (API + scanner + bot Discord)**
```
start_backend.bat
```
L'API sera disponible sur `http://localhost:8000`

**2. Lancer le frontend (interface web)**
```
start_frontend.bat
```
L'interface sera disponible sur `http://localhost:4200`

**Alternative :** `start_all.bat` lance les deux en une seule fois.

---

### 📱 Utiliser le dashboard

Rendez-vous sur **http://localhost:4200** dans votre navigateur.

#### Page Dashboard (`/dashboard`)
- **Cartes KPI** : Balance en gold, profit réalisé, montant investi, deals actifs
- **Graphique P&L** : Évolution de votre profit et de vos investissements dans le temps
- **Tableau deals récents** : Les dernières opportunités détectées
- **Bouton "Scanner l'AH"** : Déclenche manuellement un scan + analyse immédiate

> 💡 Le scan automatique tourne toutes les 60 minutes en arrière-plan.

#### Page Opportunités (`/deals`)
Liste complète de tous les deals détectés, avec filtres par statut et type.

**Pour chaque deal vous voyez :**
- **Type** : BUY (acheter maintenant) ou SELL (vendre si vous possédez l'item)
- **Prix actuel** : Le prix le plus bas actuellement en AH
- **Prix moyen (7j)** : La moyenne historique sur 7 jours
- **Marge** : Le pourcentage de profit potentiel après les frais AH
- **Confiance (0-100)** : Score de fiabilité du deal (voir explication technique plus bas)
- **Quantité suggérée** : Combien d'unités acheter en fonction de votre budget max

**Actions possibles :**
- ✅ **Exécuter** → Enregistre le deal comme acheté, ajoute l'item à votre portfolio
- ❌ **Ignorer** → Marque le deal comme skippé (n'apparaît plus dans les priorités)

#### Page Portfolio (`/portfolio`)
Suivi de tous vos investissements.

**Onglet Inventaire** : Items que vous avez achetés et pas encore vendus
- Cliquez sur "Vendre" pour pré-remplir le formulaire de vente avec le bon item

**Onglet Historique** : Toutes vos transactions (achats et ventes)

**Ajouter une transaction manuellement :**
Cliquez sur "Ajouter une transaction" pour enregistrer un achat/vente que vous avez fait directement in-game.
> ⚠️ Les prix sont en **copper** : 1 gold = 10 000 copper. Ex: 5g 50s = 55 000 copper.

#### Page Historique des Prix (`/prices/:itemId`)
Graphique de l'évolution des prix (min, moyen, médian) d'un item spécifique.
- Cliquez sur n'importe quel nom d'item dans les autres pages pour accéder à son historique
- Choisissez la période : 24h, 3j, 7j, 14j, 30j ou 90j

---

### 🤖 Le Bot Discord

Le bot Discord vous envoie des alertes dans le canal configuré (`DISCORD_CHANNEL_ID` dans `.env`) :

- 🟢 **Alerte BUY** : "Item X est à Y gold au lieu de Z gold (marge +W%)"
- 🔴 **Alerte SELL** : "Vous devriez vendre votre stock de X maintenant"
- 📊 **Résumé quotidien** : Récapitulatif des meilleures opportunités du jour

---

### ⚙️ Configurer l'application

Éditez le fichier `.env` à la racine du projet :

| Variable | Description | Valeur par défaut |
|---|---|---|
| `MIN_PROFIT_MARGIN` | Marge minimale (%) pour qu'un deal soit signalé | `20` |
| `MAX_BUDGET_GOLD` | Budget max d'investissement par cycle (en gold) | `500000` |
| `SCAN_INTERVAL_MINUTES` | Fréquence du scan automatique | `60` |
| `MIN_CONFIDENCE_SCORE` | Score de confiance minimum pour les alertes Discord | `60` |

---

## ⚙️ PARTIE TECHNIQUE

### Architecture globale

```
┌─────────────────────────────────────────────────────┐
│                     FRONTEND                         │
│  Angular 17 + PrimeNG 17  →  http://localhost:4200  │
└─────────────────────┬───────────────────────────────┘
                      │ HTTP REST (proxy /api → :8000)
┌─────────────────────▼───────────────────────────────┐
│                     BACKEND                          │
│  FastAPI 0.115  →  http://localhost:8000/api        │
│  ┌──────────────┐  ┌───────────────┐  ┌──────────┐ │
│  │  AH Scanner  │  │Trading Engine │  │Discord   │ │
│  │  (APScheduler│  │ (algorithme)  │  │Bot       │ │
│  │   60 min)    │  │               │  │          │ │
│  └──────┬───────┘  └───────┬───────┘  └──────────┘ │
│         │                  │                         │
│  ┌──────▼──────────────────▼───────────────────┐    │
│  │           SQLite Database                    │    │
│  │           ./data/wow_ah.db                   │    │
│  └──────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────┘
                      │
         ┌────────────▼────────────┐
         │   Blizzard Battle.net   │
         │   API (EU OAuth2)       │
         │   Connected Realm: 1302 │
         └─────────────────────────┘
```

---

### Stack technique

| Composant | Technologie | Version |
|---|---|---|
| Backend API | FastAPI + uvicorn | 0.115 |
| ORM | SQLAlchemy async + aiosqlite | 2.0 |
| Base de données | SQLite | — |
| HTTP client | httpx | async |
| Scheduler | APScheduler | 3.10 |
| Bot Discord | discord.py | 2.4 |
| Config | pydantic-settings | 2.x |
| Frontend | Angular standalone | 17 |
| UI Components | PrimeNG | 17 |
| CSS Layout | PrimeFlex | 3 |
| Charts | Chart.js via p-chart | 4 |
| Thème | lara-dark-indigo | — |

---

### Schéma de la base de données

```
Item                    AuctionSnapshot           AuctionEntry
────────────           ─────────────────        ──────────────────
id (PK)                id (PK)                  id (PK)
name                   scanned_at               snapshot_id (FK)
icon_url               total_auctions           item_id (FK)
                       duration_ms              quantity
                                                unit_price
                                                buyout

PriceHistory           Deal                     Portfolio           GoldBalance
────────────           ─────────────────        ──────────────────  ───────────────
id (PK)                id (PK)                  id (PK)             id (PK)
item_id (FK)           item_id (FK)             item_id (FK)        recorded_at
scanned_at             detected_at              action (BUY/SELL)   gold_copper
min_buyout             deal_type (BUY/SELL)     quantity            invested_copper
avg_buyout             current_price            price_per_unit      profit_copper
median_buyout          avg_price                total_price
total_quantity         profit_margin            created_at
num_auctions           confidence_score         notes
                       suggested_quantity
                       status (PENDING/...)
```

---

### Pipeline de scan (AH Scanner)

Exécuté toutes les `SCAN_INTERVAL_MINUTES` minutes par APScheduler :

```
1. BlizzardAuth.get_auth_headers()
   └─ OAuth2 client_credentials → token Blizzard (mis en cache)

2. BlizzardAPI.get_auctions(connected_realm_id=1302)
   └─ GET https://eu.api.blizzard.com/data/wow/connected-realm/1302/auctions
   └─ Retourne jusqu'à ~100 000 enchères actives

3. BlizzardAPI.get_commodities()
   └─ GET https://eu.api.blizzard.com/data/wow/auctions/commodities
   └─ Articles en vente de masse (minerais, herbes, etc.)

4. AHScanner.scan()
   ├─ Crée un AuctionSnapshot (timestamp + métadonnées)
   ├─ Insère chaque AuctionEntry en base
   ├─ Agrège par item_id → PriceHistory (min, avg, median, quantité)
   └─ Enrichit les Item manquants via get_item_with_details()
      └─ GET https://eu.api.blizzard.com/data/wow/item/{item_id}
```

---

### Algorithme de détection de deals (Trading Engine)

```python
# Pour chaque item ayant >= 3 points d'historique sur 7 jours :

# 1. Calcul du prix de référence
avg_7d   = moyenne des prix min sur 7 jours
std_dev  = écart-type des prix sur 7 jours
current  = prix minimum actuel en AH

# 2. Calcul de la marge brute
margin = ((avg_7d - current) / avg_7d) * 100

# 3. Application des frais AH (5%)
margin_net = margin - 5

# 4. Filtre : seulement si margin_net >= MIN_PROFIT_MARGIN (défaut: 20%)

# 5. Score de confiance (0-100) = somme de 4 composantes :
confidence_score = (
  margin_component   # 0-40 pts : selon la marge nette
  + stability        # 0-25 pts : +25 si std_dev < 15% de avg_7d
  + liquidity        # 0-20 pts : +20 si nb_auctions > 10
  + data_quality     # 0-15 pts : +15 si >= 7 points d'historique
)

# 6. Calcul de la quantité suggérée
budget_copper      = MAX_BUDGET_GOLD * 10000
suggested_quantity = min(floor(budget_copper / current_price), stock_disponible)

# 7. Création du Deal en base (status=PENDING)
# 8. Si confidence_score >= MIN_CONFIDENCE_SCORE → alerte Discord
```

---

### API REST — Endpoints disponibles

| Méthode | URL | Description |
|---|---|---|
| `GET` | `/api/dashboard` | Résumé global (KPIs, deals récents, historique gold) |
| `GET` | `/api/deals` | Liste des deals (filtres: status, type, limit) |
| `POST` | `/api/deals/{id}/execute` | Marque un deal comme exécuté (PENDING → EXECUTED) |
| `POST` | `/api/deals/{id}/skip` | Ignore un deal (PENDING → SKIPPED) |
| `GET` | `/api/portfolio` | Historique des transactions |
| `POST` | `/api/portfolio` | Ajoute une transaction manuelle |
| `GET` | `/api/portfolio/inventory` | Inventaire actuel (items non vendus) |
| `GET` | `/api/portfolio/pnl` | Résumé P&L (investi, revenus, frais, profit net) |
| `GET` | `/api/gold-history` | Historique de la balance gold |
| `GET` | `/api/prices/{item_id}` | Historique des prix d'un item |
| `GET` | `/api/items/search?q=...` | Recherche d'items par nom |
| `POST` | `/api/scan` | Déclenche un scan manuel |
| `POST` | `/api/analyze` | Déclenche une analyse manuelle |

Documentation interactive disponible sur `http://localhost:8000/docs` (Swagger UI).

---

### Configuration Blizzard API

L'accès à l'API Blizzard utilise le flow **OAuth2 Client Credentials** :

```
POST https://oauth.battle.net/token
  grant_type=client_credentials
  client_id=<BLIZZARD_CLIENT_ID>
  client_secret=<BLIZZARD_CLIENT_SECRET>

→ access_token (valide 24h, mis en cache automatiquement)
```

- **Region** : EU (`eu.api.blizzard.com`)
- **Connected Realm ID** : `1302` (Archimonde EU — vérifié via l'API)
- **Realm Slug** : `archimonde`
- **Game version** : Retail (namespace `dynamic-eu`)

> Les tokens sont automatiquement renouvelés avant expiration.

---

### Structure des fichiers

```
WOW/
├── .env                        # Configuration (credentials, tokens)
├── .env.example                # Template sans données sensibles
├── .gitignore                  # .env + data/ + __pycache__ exclus
├── README.md                   # Guide d'installation rapide
├── FONCTIONNEMENT.md           # Ce fichier
├── install.bat                 # Installation des dépendances (1 fois)
├── start_backend.bat           # Démarrage API + bot
├── start_frontend.bat          # Démarrage interface web
├── start_all.bat               # Démarrage tout
│
├── backend/
│   ├── main.py                 # Point d'entrée FastAPI (CORS, lifespan)
│   ├── requirements.txt
│   ├── core/
│   │   ├── config.py           # Settings (pydantic-settings)
│   │   ├── auth.py             # OAuth2 Blizzard
│   │   └── database.py         # SQLAlchemy async, init_db()
│   ├── models/
│   │   ├── models.py           # Tables SQLAlchemy (7 tables)
│   │   └── schemas.py          # Schémas Pydantic (réponses API)
│   ├── services/
│   │   ├── blizzard_api.py     # Client HTTP Blizzard
│   │   ├── ah_scanner.py       # Pipeline de scan
│   │   ├── trading_engine.py   # Algorithme de détection
│   │   └── portfolio_service.py # Gestion portfolio/P&L
│   ├── bot/
│   │   └── discord_bot.py      # Bot Discord (alertes)
│   ├── scheduler/
│   │   └── jobs.py             # APScheduler (scan périodique)
│   └── api/
│       └── routes.py           # Tous les endpoints REST
│
└── frontend/
    ├── package.json
    ├── angular.json
    ├── proxy.conf.json         # /api → localhost:8000
    └── src/
        ├── main.ts
        ├── index.html
        ├── styles.scss         # Styles globaux PrimeNG
        └── app/
            ├── app.config.ts   # Configuration Angular (providers)
            ├── app.routes.ts   # Routes
            ├── app.component.ts # Layout sidebar + topbar
            ├── models/
            │   └── interfaces.ts # Types TypeScript
            ├── services/
            │   └── api.service.ts # Calls HTTP vers le backend
            ├── pipes/
            │   └── gold-format.pipe.ts # Formatage copper → Xg Ys Zc
            └── pages/
                ├── dashboard/      # Page principale
                ├── deals/          # Liste des opportunités
                ├── portfolio/      # Suivi investissements
                └── price-history/  # Graphiques de prix
```

---

### Sécurité & données sensibles

- Le fichier `.env` contient les credentials Blizzard et le token Discord → **ne jamais commiter**
- `.gitignore` exclut `.env`, `data/` (base SQLite), `__pycache__`, `node_modules`
- `.env.example` contient uniquement des placeholders sans valeur réelle
