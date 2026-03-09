# 🏗️ WoW AH Trading Bot — Architecture & Structure

> **Référence vivante** : ce fichier décrit la structure exacte, le rôle de chaque fichier, les modèles de données, les flux et la logique métier.
> Toute modification de structure doit se refléter dans le code, et inversement.

---

## 📍 Vue d'ensemble

Bot de trading automatisé pour l'Hôtel des Ventes (AH) de World of Warcraft.
Il scanne l'AH du serveur **Archimonde (EU)**, détecte les opportunités d'achat/revente,
envoie des alertes Discord, et expose un dashboard web Angular.

| Composant | Techno | Port |
|---|---|---|
| Backend API | **Go 1.22** (Gin + GORM + SQLite) | `8000` |
| Frontend | **Angular Latest version** (standalone) + PrimeNG 17 | `4200` |
| Bot Discord | **discordgo** (intégré au backend Go) | — |
| Scheduler | **gocron** (intégré au backend Go) | — |
| Base de données | **SQLite** (WAL mode, busy_timeout=30s) | — |
| API externe | **Blizzard API** (OAuth2 client_credentials) | — |

---

## 📁 Arborescence des fichiers

```
WOW/
├── .env.example                          # Template des variables d'environnement
├── STRUCTURE.md                          # CE FICHIER — référence d'architecture
│
├── backend-go/                           # ═══ BACKEND GO ═══
│   ├── .env.example                      # Template .env spécifique Go
│   ├── .env                              # Variables d'environnement (non versionné)
│   ├── go.mod                            # Dépendances Go
│   ├── go.sum                            # Checksums des dépendances
│   ├── wow-ah-bot.exe                    # Binaire compilé (non versionné)
│   │
│   ├── cmd/
│   │   └── server/
│   │       └── main.go                   # Point d'entrée — boot séquence
│   │
│   ├── data/
│   │   └── wow_ah.db                     # Base SQLite (créée automatiquement)
│   │
│   └── internal/
│       ├── config/
│       │   └── config.go                 # Chargement .env → singleton Settings
│       │
│       ├── database/
│       │   └── database.go               # Connexion GORM + auto-migration
│       │
│       ├── models/
│       │   ├── models.go                 # 10 modèles ORM (tables SQLite)
│       │   └── dto.go                    # DTOs JSON + utilitaires (CopperToGoldStr, RoundFloat)
│       │
│       ├── services/
│       │   ├── blizzard/
│       │   │   └── client.go             # OAuth2 + API Blizzard (enchères, items, icônes)
│       │   ├── scanner/
│       │   │   └── scanner.go            # Scan AH complet + cache items concurrent
│       │   ├── trading/
│       │   │   └── engine.go             # Analyse de marché + Indice de Rentabilité (IR)
│       │   ├── character/
│       │   │   └── service.go            # CRUD personnages + transactions + P&L + snapshots
│       │   └── portfolio/
│       │       └── portfolio.go          # P&L global, inventaire, transactions
│       │
│       ├── api/
│       │   └── routes.go                 # 27 routes REST Gin
│       │
│       ├── discord/
│       │   └── bot.go                    # Bot Discord + embeds riches
│       │
│       └── scheduler/
│           └── scheduler.go              # Job périodique scan+analyze
│
└── frontend/                             # ═══ FRONTEND ANGULAR ═══
    ├── angular.json                      # Config Angular CLI
    ├── package.json                      # Dépendances npm
    ├── proxy.conf.json                   # Proxy dev → localhost:8000
    ├── tsconfig.json                     # Config TypeScript
    └── src/
        ├── environments/
        │   └── environment.ts            # apiUrl = "/api"
        └── app/
            ├── app.component.ts          # Shell + navigation (sidebar PrimeNG)
            ├── app.config.ts             # Providers Angular (HttpClient, animations)
            ├── app.routes.ts             # 4 routes lazy-loaded
            ├── models/
            │   └── interfaces.ts         # 8 interfaces TypeScript (= DTOs Go)
            ├── services/
            │   └── api.service.ts        # HttpClient vers les 14 endpoints
            ├── pipes/                    # Pipes custom (gold formatting, etc.)
            └── pages/
                ├── dashboard/            # Vue d'ensemble P&L + deals récents
                ├── deals/                # Grille de cards avec images items
                ├── portfolio/            # Transactions + inventaire
                └── price-history/        # Graphique prix d'un item
```

---

## ⚡ Séquence de démarrage (`main.go`)

```
1. config.Init()           → Charge .env (backend-go/.env puis WOW/.env en fallback)
2. database.Init()         → Ouvre SQLite, auto-migrate les 7 tables
3. discord.New() + Start() → Connexion bot Discord en goroutine
4. scheduler.Start(bot)    → Lance le job cron (scan+analyze toutes les N minutes)
5. api.NotifyFunc = ...    → Wire la callback Discord dans les routes API (évite l'import circulaire)
6. gin.New() + CORS        → Serveur HTTP avec CORS pour Angular
7. Signal handler          → Graceful shutdown sur SIGINT/SIGTERM
8. r.Run(addr)             → Écoute sur 0.0.0.0:8000
```

---

## 🗄️ Modèles de données (7 tables SQLite)

### `items` — Cache des items WoW (depuis Blizzard API)
| Colonne | Type | Description |
|---|---|---|
| `id` | INT PK | ID Blizzard de l'item |
| `name` | VARCHAR(255) | **Vrai nom** localisé (fr_FR) depuis l'API Blizzard |
| `quality` | VARCHAR(50) | Rareté : COMMON, UNCOMMON, RARE, EPIC, LEGENDARY |
| `item_class` | VARCHAR(100) | Classe (Arme, Armure, Consommable, etc.) |
| `item_subclass` | VARCHAR(100) | Sous-classe (Épée, Potion, etc.) |
| `level` | INT | Niveau requis |
| `icon_url` | VARCHAR(500) | URL CDN de l'icône (render.worldofwarcraft.com) |
| `is_commodity` | BOOL | true = commodity (stackable, prix global) |
| `vendor_price` | INT64 | Prix de vente au vendeur (copper) |
| `updated_at` | DATETIME | Dernière mise à jour du cache |

### `auction_snapshots` — Historique des scans
| Colonne | Type | Description |
|---|---|---|
| `id` | UINT PK AUTO | ID du snapshot |
| `scanned_at` | DATETIME INDEX | Horodatage du scan |
| `total_auctions` | INT | Nombre total d'enchères dans ce scan |
| `total_gold_volume` | INT64 | Volume total en copper de toutes les enchères |

### `auction_entries` — Enchères individuelles par snapshot
| Colonne | Type | Description |
|---|---|---|
| `id` | UINT PK AUTO | ID interne |
| `snapshot_id` | UINT INDEX | FK → auction_snapshots |
| `auction_id` | INT64 | ID Blizzard de l'enchère |
| `item_id` | INT INDEX | FK → items |
| `quantity` | INT | Quantité |
| `unit_price` | INT64 | Prix unitaire (copper) |
| `buyout` | INT64 | Prix buyout (copper) |
| `bid` | INT64 | Enchère actuelle (copper) |
| `time_left` | VARCHAR(20) | SHORT, MEDIUM, LONG, VERY_LONG |

### `price_history` — Agrégats de prix par item par scan
| Colonne | Type | Description |
|---|---|---|
| `id` | UINT PK AUTO | ID interne |
| `item_id` | INT INDEX | FK → items |
| `scanned_at` | DATETIME INDEX | Horodatage |
| `min_buyout` | INT64 | Prix minimum (copper) |
| `avg_buyout` | INT64 | Prix moyen (copper) |
| `median_buyout` | INT64 | Prix médian (copper) |
| `max_buyout` | INT64 | Prix maximum (copper) |
| `total_quantity` | INT | Quantité totale en vente |
| `num_auctions` | INT | Nombre d'enchères |

### `deals` — Opportunités de trading détectées
| Colonne | Type | Description |
|---|---|---|
| `id` | UINT PK AUTO | ID du deal |
| `item_id` | INT INDEX | FK → items |
| `item_name` | VARCHAR(255) | Nom de l'item (copie dénormalisée) |
| `detected_at` | DATETIME | Date de détection |
| `deal_type` | VARCHAR(20) | `BUY` ou `SELL` |
| `current_price` | INT64 | Prix actuel min (copper) |
| `avg_price` | INT64 | Prix moyen historique 7j (copper) |
| `suggested_buy_price` | INT64 | Prix d'achat recommandé (copper) |
| `suggested_sell_price` | INT64 | Prix de vente recommandé (copper) |
| `suggested_quantity` | INT | Quantité recommandée |
| `profit_margin` | FLOAT64 | Marge de profit estimée (%) |
| `confidence_score` | FLOAT64 | Score de confiance (0-100) |
| `status` | VARCHAR(20) | `PENDING`, `EXECUTED`, `SKIPPED`, `EXPIRED` |
| `notified` | BOOL | true = déjà envoyé sur Discord |

### `portfolios` — Transactions réelles (achats/ventes)
| Colonne | Type | Description |
|---|---|---|
| `id` | UINT PK AUTO | ID transaction |
| `item_id` | INT INDEX | FK → items |
| `item_name` | VARCHAR(255) | Nom de l'item |
| `action` | VARCHAR(10) | `BUY` ou `SELL` |
| `quantity` | INT | Quantité |
| `price_per_unit` | INT64 | Prix unitaire (copper) |
| `total_price` | INT64 | Prix total = price_per_unit × quantity |
| `created_at` | DATETIME | Date de la transaction |
| `notes` | VARCHAR(500) | Notes libres |

### `gold_balance` — Snapshots de solde pour graphiques P&L
| Colonne | Type | Description |
|---|---|---|
| `id` | UINT PK AUTO | ID |
| `recorded_at` | DATETIME INDEX | Horodatage |
| `balance_copper` | INT64 | Solde net (copper) |
| `invested_copper` | INT64 | Capital investi (copper) |
| `profit_copper` | INT64 | Profit réalisé (copper) |

---

## 🔌 API REST — 14 endpoints

Toutes les routes sont préfixées par `/api`. Le serveur écoute sur le port `8000`.

### Dashboard
| Méthode | Route | Description | Réponse |
|---|---|---|---|
| `GET` | `/api/dashboard` | Vue d'ensemble P&L + deals récents | `DashboardSummaryDTO` |

**Logique** : agrège P&L (somme achats vs ventes - 5% frais AH), compte les deals PENDING, récupère les 10 derniers deals avec noms d'items résolus via table `items`, calcule l'investissement courant depuis l'inventaire.

### Deals
| Méthode | Route | Description | Réponse |
|---|---|---|---|
| `GET` | `/api/deals` | Liste des deals (filtrable) | `[]DealDTO` |
| `POST` | `/api/deals/:id/execute` | Exécuter un deal → portfolio | `{message}` |
| `POST` | `/api/deals/:id/skip` | Ignorer un deal | `{message}` |

**Paramètres GET /deals** : `?status=PENDING&deal_type=BUY&limit=100`

**Logique GET /deals** : JOIN `deals` ← LEFT JOIN `items` pour récupérer le vrai nom (`items.name`) et l'icône (`items.icon_url`). Trié par `profit_margin × confidence_score DESC`.

**Logique execute** : crée une entrée `Portfolio` avec le prix suggéré, met le deal en status `EXECUTED`, met à jour le gold_balance.

### Portfolio
| Méthode | Route | Description | Réponse |
|---|---|---|---|
| `GET` | `/api/portfolio` | Transactions récentes | `[]PortfolioDTO` |
| `POST` | `/api/portfolio` | Ajouter une transaction | `PortfolioDTO` |
| `GET` | `/api/portfolio/inventory` | Inventaire actuel (achetés - vendus) | `[]InventoryItem` |
| `GET` | `/api/portfolio/pnl` | Résumé Profit & Loss | `PnlSummaryDTO` |

**Logique P&L** : `profit = total_vendu - (total_vendu × 5% frais AH) - total_acheté`

**Logique inventaire** : GROUP BY item_id sur les BUY (SUM quantity, AVG price) et les SELL (SUM quantity, SUM revenue), puis `remaining = bought - sold`.

### Historique
| Méthode | Route | Description | Réponse |
|---|---|---|---|
| `GET` | `/api/gold-history` | Historique du solde gold | `[]GoldBalanceDTO` |
| `GET` | `/api/prices/:itemId` | Historique des prix d'un item | `PriceHistoryListDTO` |

**Paramètres** : `?days=30` (gold-history) / `?days=7` (prices)

### Items
| Méthode | Route | Description | Réponse |
|---|---|---|---|
| `GET` | `/api/items/search` | Recherche d'items par nom | `[]ItemDTO` |

**Paramètres** : `?q=potion&limit=20` (min 2 caractères)

### Actions
| Méthode | Route | Description | Réponse |
|---|---|---|---|
| `POST` | `/api/scan` | Lancer un scan AH seul | `{message, total_auctions, scanned_at}` |
| `POST` | `/api/analyze` | Lancer une analyse seule | `{message, deals_count}` |
| `POST` | `/api/refresh` | Scan + Analyse + Notif Discord | `{message, total_auctions, unique_items, deals_count, scanned_at}` |

**Logique refresh** : `Scan() → Analyze() → Discord notify (goroutine non-bloquante)`

---

## 📊 DTOs (Data Transfer Objects)

Les DTOs sont les structures JSON envoyées au frontend. Ils enrichissent les modèles ORM avec des champs calculés (prix en gold, profit, icônes).

### DealDTO
Enrichit `Deal` avec :
- `icon_url` : depuis la table `items` (JOIN)
- `item_name` : priorité `items.name` > `deals.item_name` > `"Item #ID"` (dernier recours)
- `current_price_gold`, `avg_price_gold`, `suggested_buy_price_gold`, `suggested_sell_price_gold` : conversion copper→gold via `CopperToGoldStr()`
- `potential_profit_gold` : `(sell - buy) × quantity × 0.95` (avec 5% frais AH)

### Autres DTOs
- **PortfolioDTO** : ajoute `total_price_gold`
- **GoldBalanceDTO** : ajoute `balance_gold`, `profit_gold`
- **PriceHistoryDTO** : ajoute `min_buyout_gold`, `avg_buyout_gold`
- **DashboardSummaryDTO** : agrège P&L, deals récents, gold history
- **PnlSummaryDTO** : invested, revenue, fees, profit (copper + gold)
- **ItemDTO** : projection légère d'un Item pour la recherche

### Utilitaire `CopperToGoldStr(copper int64) string`
Convertit un montant en copper vers une chaîne lisible : `150000` → `"15g"`, `12345` → `"1g 23s 45c"`, `-50000` → `"-5g"`.

---

## 🔄 Flux principaux

### Flux 1 : Scan AH automatique (scheduler, toutes les N minutes)

```
┌─────────────┐    ┌──────────────┐    ┌───────────────┐    ┌─────────────┐
│  Scheduler   │───▶│ scanner.Scan │───▶│trading.Analyze│───▶│ Discord Bot │
│  (gocron)    │    │              │    │               │    │             │
│  every 60min │    │ 1. Blizzard  │    │ 1. Query 7j   │    │ SendScan    │
│              │    │    GetAuct.  │    │    history     │    │  Report()   │
│              │    │ 2. Snapshot  │    │ 2. Deviation   │    │             │
│              │    │ 3. Entries   │    │    vs average  │    │ SendDeals   │
│              │    │    batch ins │    │ 3. Confidence  │    │  Summary()  │
│              │    │ 4. Price agg │    │    scoring     │    │             │
│              │    │ 5. Item cache│    │ 4. Persist     │    │ Mark deals  │
│              │    │    (5 workers│    │    deals       │    │  notified   │
│              │    │    concurrent│    │               │    │             │
│              │    │    sync)     │    │               │    │             │
└─────────────┘    └──────────────┘    └───────────────┘    └─────────────┘
```

### Flux 2 : Refresh manuel (bouton frontend ou auto-refresh 60s)

```
┌──────────┐   POST /api/refresh   ┌──────────────┐    ┌───────────────┐
│ Angular  │──────────────────────▶│ routes.go     │───▶│ scanner.Scan  │
│ Frontend │                       │ refreshAll()  │    │ + Analyze()   │
│          │◀──────────────────────│               │    │               │
│          │   JSON response       │ go NotifyFunc │───▶│ Discord (async)
└──────────┘                       └──────────────┘    └───────────────┘
```

### Flux 3 : Résolution des noms d'items (3 niveaux)

Le système garantit l'affichage du **vrai nom** de l'item (jamais "Item #2901") via 3 niveaux :

```
Niveau 1 — Scanner (au moment du scan)
  └─ updateUnknownItems() : 5 goroutines fetchent en parallèle les items inconnus
     via Blizzard API (GetItemWithDetails), upsert dans table items.
     SYNCHRONE : bloque jusqu'à ce que tous les noms soient cachés.
     Limite : 100 items par scan. Upsert écrase les noms vides.

Niveau 2 — Trading Engine (au moment de l'analyse)
  └─ buildDeal() : si le nom n'est pas en DB, appelle fetchAndCacheItemName()
     qui fait GetItemWithDetails() + db.Save() et retourne le vrai nom.

Niveau 3 — Routes API (au moment de la réponse HTTP)
  └─ resolveItem() : si le nom n'est pas en DB, appelle GetItemWithDetails()
     et cache le résultat. Dernier filet de sécurité avant la réponse JSON.
     Fallback ultime : "Item #ID" (ne devrait jamais arriver).
```

### Flux 4 : Exécution d'un deal

```
Frontend: POST /api/deals/42/execute
  └─ routes.go executeDeal()
       ├─ Charge Deal #42 depuis DB
       ├─ portfolio.AddTransaction(item_id, name, BUY, qty, price, notes)
       │    ├─ Crée Portfolio entry
       │    └─ go updateBalance() → crée GoldBalance snapshot
       └─ Met Deal.status = "EXECUTED"
```

---

## 🧠 Logique métier — Trading Engine (Indice de Rentabilité)

### Constantes
| Paramètre | Valeur | Description |
|---|---|---|
| `minHistoryPoints` | 5 | Nombre min de scans pour scorer un item |
| `lookbackDays` | 7 | Fenêtre d'historique (jours) |
| `minDailyVolume` | 10 | Volume quotidien min pour être éligible |
| `ahCut` | 0.05 (5%) | Commission de l'Hôtel des Ventes |
| `minIRForDeal` | 40.0 | Seuil IR minimum pour créer un Deal |

### Indice de Rentabilité (IR) — Score 0-100 par item

Chaque scan crée une ligne dans `item_scores` pour chaque item éligible,
composée de **5 composantes indépendantes** pondérées :

| Composante | Poids | Calcul | Interprétation |
|---|---|---|---|
| **Sous-évaluation** | **35%** | `(histMedian - currentMin) / histMedian`, cap à 50% | Plus le prix actuel est bas vs la médiane historique, mieux c'est |
| **Momentum négatif** | **20%** | Pente OLS sur prix min historiques, normalisée par prix moyen | Pente négative = prix en baisse = bon moment d'entrée |
| **Liquidité** | **20%** | `avgVolume / 200`, cap à 100 | Volume quotidien élevé = item facile à revendre |
| **Stabilité** | **15%** | `1 - CV/0.5` où CV = `stddev/avg` | Prix stable = plus prévisible, moins risqué |
| **Profit net AH** | **10%** | `(histMedian - currentMin) × 0.95 / currentMin`, cap à 50% | Marge nette après commission AH |

**IR final** = somme des composantes × leurs poids (toujours dans [0, 100])

### Pipeline Analyze()

1. **Récupère** l'ID du dernier snapshot (sert de `scan_id` pour regrouper les scores)
2. **Filtre** les items avec ≥ 5 points d'historique sur 7 jours
3. Pour chaque item éligible : **`scoreItem()`** → calcule les 5 composantes + IR
4. Persiste tous les `ItemScore` en batch (écrase les scores du même `scan_id`)
5. Si IR ≥ `minIRForDeal` ET marge ≥ `MIN_PROFIT_MARGIN` : **`buildDeal()`** → crée un Deal
6. Trie les deals par IR décroissant
7. Persiste les deals en batch

### Table `item_scores` — structure
```
item_scores
├─ id, item_id, scan_id (unique pair)    → une ligne par item par scan
├─ scored_at                              → timestamp du scoring
├─ score_undervaluation  (0-100)          → composante sous-évaluation
├─ score_momentum        (0-100)          → composante momentum
├─ score_liquidity       (0-100)          → composante liquidité
├─ score_stability       (0-100)          → composante stabilité
├─ score_net_profit      (0-100)          → composante profit net
├─ rentability_index     (0-100)          → IR final pondéré
├─ current_min_price                      → prix min au moment du score
├─ hist_median_price                      → médiane historique utilisée
├─ avg_daily_volume                       → volume quotidien moyen
├─ price_slope                            → pente OLS brute des prix
├─ coeff_variation                        → CV = stddev/avg
└─ data_points                            → nombre de scans utilisés
```

---

## 🤖 Bot Discord

### Connexion
- Librairie : `discordgo`
- Intents : `IntentsGuildMessages`
- Channel ID : configuré dans `.env` (`DISCORD_CHANNEL_ID`)
- `waitReady()` : bloque max 10s que le bot soit connecté (via channel `ready`)
- Si token absent → bot désactivé silencieusement

### Messages envoyés

**1. Scan Report** (après chaque scan) :
- Embed blurple (0 deals) ou vert (≥1 deal)
- Champs : enchères analysées, items uniques, nouvelles opportunités, durée, prochain scan

**2. Deals Summary** (si ≥1 nouveau deal) :
- Embed doré (header) : "X deal(s) détecté(s) à HH:MM UTC"
- Jusqu'à 10 embeds individuels (1 par deal) avec :
  - Titre : 🛒 ACHETER + **vrai nom de l'item**
  - Couleur selon IR : vert (≥80), or (≥60), rose (≥40), blurple (<40)
  - Barre de progression IR : 🟩🟩🟩🟩🟩🟩🟩⬜⬜⬜ **73.5 / 100**
  - Prix actuel, prix médian 7j, marge estimée
  - Action recommandée (qty × prix d'achat → prix de revente)
  - Investissement + profit potentiel net AH
- Si >10 deals : message texte "... et N autres deals"
- Rate limit : 500ms entre chaque embed

### Intégration
- **Scheduler** : appelle directement `bot.SendScanReport()` + `bot.SendDealsSummary()`
- **Route /refresh** : via `api.NotifyFunc` (callback injectée par `main.go` pour éviter import circulaire), en goroutine non-bloquante

---

## 🌐 Frontend Angular

### Stack
- Angular 17 standalone (pas de NgModule)
- PrimeNG 17 — thème `lara-light-blue`
- Routing lazy-loaded (4 pages)
- Proxy dev : `proxy.conf.json` → `http://localhost:8000`

### Pages

**1. Dashboard** (`/dashboard`)
- Résumé P&L (investi, profit, solde)
- Nombre de deals actifs + items suivis
- Date du dernier scan
- Graphique gold history (30j)
- Tableau des 10 derniers deals
- Auto-refresh toutes les 60 secondes (appelle `/api/refresh` puis `/api/dashboard`)

**2. Deals** (`/deals`)
- Grille de cards responsive
- Chaque card affiche :
  - Icône de l'item (depuis `icon_url`)
  - **Vrai nom** de l'item (depuis `item_name`)
  - Prix actuel vs moyen en gold
  - Marge + confiance
  - Boutons "Exécuter" et "Ignorer"
- Filtrage par status et type
- Tri par `profit_margin × confidence_score` (fait côté backend)
- Auto-refresh toutes les 60 secondes

**3. Portfolio** (`/portfolio`)
- Liste des transactions (BUY/SELL)
- Inventaire actuel (items en stock)
- Résumé P&L

**4. Price History** (`/prices/:itemId`)
- Graphique de l'évolution des prix d'un item
- Min, avg, median buyout sur N jours

**5. Characters** (`/characters`)
- Liste des personnages suivis (nom, realm, classe, race, niveau, avatar)
- Bouton ajout / modification / suppression
- Badge actif/inactif

**6. Character Detail** (`/characters/:id`)
- Carte d'identité du personnage
- P&L complet (investi, vendu, frais AH, profit réalisé, positions ouvertes)
- Win rate (% de ventes bénéficiaires)
- Solde actuel (via dernier snapshot)
- Graphique de richesse dans le temps
- Historique des transactions (BUY/SELL avec item, prix, date, lien deal)
- Bouton "Enregistrer solde" pour créer un snapshot manuel

### Interfaces TypeScript (8 interfaces dans `interfaces.ts`)

Les interfaces correspondent **exactement** aux DTOs Go (mêmes clés JSON snake_case) :

| Interface TS | DTO Go | Champs clés |
|---|---|---|
| `DashboardSummary` | `DashboardSummaryDTO` | gold strings, active_deals, gold_history[], recent_deals[] |
| `Deal` | `DealDTO` | item_name, icon_url, profit_margin, **rentability_index**, *_gold |
| `ItemScore` | `ItemScoreDTO` | score_undervaluation, score_momentum, score_liquidity, score_stability, score_net_profit, rentability_index, weights |
| `PortfolioEntry` | `PortfolioDTO` | action, quantity, total_price_gold |
| `GoldBalance` | `GoldBalanceDTO` | balance_gold, profit_gold |
| `PriceHistoryEntry` | `PriceHistoryDTO` | min_buyout_gold, avg_buyout_gold |
| `PriceHistoryList` | `PriceHistoryListDTO` | item_name, history[] |
| `InventoryItem` | `InventoryItem` (Go struct directe) | avg_buy_price, quantity |
| `PnlSummary` | `PnlSummaryDTO` | invested, revenue, fees, profit |
| `WowItem` | `ItemDTO` | name, icon_url, quality, is_commodity |
| `Character` | `CharacterDTO` | name, realm, class, race, level, avatar_url, is_active |
| `CharacterTransaction` | `CharacterTransactionDTO` | action, item_name, icon_url, price_per_unit_gold, total_price_gold, deal_id |
| `CharacterSnapshot` | `CharacterSnapshotDTO` | balance_gold, invested_gold, profit_gold |
| `CharacterPnl` | `CharacterPnlDTO` | total_buy_gold, realized_pnl_gold, open_position_gold, win_rate, latest_balance_gold |

### Service API (`api.service.ts`)

14 méthodes correspondant exactement aux 14 routes Go :

```typescript
getDashboard()                               → GET    /api/dashboard
getDeals(status?, limit)                     → GET    /api/deals               (trié par IR DESC)
executeDeal(id)                              → POST   /api/deals/:id/execute
skipDeal(id)                                 → POST   /api/deals/:id/skip
getPortfolio(limit)                          → GET    /api/portfolio
addPortfolioEntry(entry)                     → POST   /api/portfolio
getInventory()                               → GET    /api/portfolio/inventory
getPnl()                                     → GET    /api/portfolio/pnl
getGoldHistory(days)                         → GET    /api/gold-history
getPriceHistory(itemId, days)                → GET    /api/prices/:itemId
searchItems(query, limit)                    → GET    /api/items/search
getItemScores(limit)                         → GET    /api/scores              (dernier IR par item)
getItemScoreHistory(itemId, limit)           → GET    /api/scores/:itemId      (historique IR)
// Personnages
listCharacters()                             → GET    /api/characters
createCharacter(req)                         → POST   /api/characters
getCharacter(id)                             → GET    /api/characters/:id
updateCharacter(id, req)                     → PUT    /api/characters/:id
deleteCharacter(id)                          → DELETE /api/characters/:id
getCharacterPnl(id)                          → GET    /api/characters/:id/pnl
getCharacterTransactions(id, limit)          → GET    /api/characters/:id/transactions
addCharacterTransaction(id, req)             → POST   /api/characters/:id/transactions
getCharacterSnapshots(id, days)              → GET    /api/characters/:id/snapshots
addCharacterSnapshot(id, req)                → POST   /api/characters/:id/snapshots
// Actions
triggerScan()                                → POST   /api/scan
triggerAnalysis()                            → POST   /api/analyze
triggerRefresh()                             → POST   /api/refresh
```

---

## 🧙 Système de personnages (`internal/services/character/service.go`)

### Objectif
Suivre l'activité AH de chaque personnage WoW de façon **indépendante** du portfolio global.
Chaque personnage a son propre historique de transactions, son P&L et son évolution de richesse.

### Tables impliquées
| Table | Rôle |
|---|---|
| `characters` | Fiche du personnage (nom, realm, classe, race, niveau, avatar, actif/inactif) |
| `character_transactions` | Chaque achat/vente AH du personnage — avec lien optionnel vers un Deal détecté |
| `character_snapshots` | Snapshot journalier de la richesse (balance, investi, profit) |

### Champs clés de `character_transactions`
```
character_id   → FK vers characters.id
item_id        → référence à l'item WoW
action         → BUY ou SELL
quantity       → nombre d'unités
price_per_unit → prix unitaire en copper
total_price    → quantité × prix_unitaire
deal_id        → (optionnel) lie la transaction à un Deal détecté par le bot
transacted_at  → timestamp de la transaction
```

### P&L par personnage (`GetPnl`)
```
total_buy     = SUM(total_price) WHERE action=BUY
total_sell    = SUM(total_price) WHERE action=SELL
ah_fees       = total_sell × 5%
realized_pnl  = total_sell - ah_fees - total_buy

open_positions = items encore en stock (buy_qty - sell_qty) × avg_buy_price

win_rate = % d'items vendus avec marge positive
           (avg_sell_price > avg_buy_price pour cet item)
```

### Snapshots
- Créés automatiquement (en goroutine) après chaque transaction
- Créés manuellement via `POST /api/characters/:id/snapshots` (avec le solde gold actuel)
- Permettent de tracer l'évolution de la richesse dans le temps (graphique frontend)

### Lien avec les Deals
Quand on exécute un deal via `POST /api/deals/:id/execute`, on peut passer un `character_id`
dans le body → la transaction est automatiquement créée dans `character_transactions`
avec `deal_id` rempli, permettant de tracer les deals exécutés par personnage.

---

## ⚙️ Configuration (.env)

| Variable | Défaut | Description |
|---|---|---|
| `BLIZZARD_CLIENT_ID` | — | Client ID OAuth2 (https://develop.battle.net/) |
| `BLIZZARD_CLIENT_SECRET` | — | Client Secret OAuth2 |
| `BLIZZARD_REGION` | `eu` | Région : eu, us, kr, tw |
| `BLIZZARD_REALM_SLUG` | `archimonde` | Slug du serveur |
| `BLIZZARD_CONNECTED_REALM_ID` | `1302` | ID du connected realm |
| `BLIZZARD_LOCALE` | `fr_FR` | Locale pour les noms d'items |
| `DISCORD_BOT_TOKEN` | — | Token du bot Discord |
| `DISCORD_CHANNEL_ID` | — | ID du channel Discord pour les alertes |
| `DATABASE_PATH` | `./data/wow_ah.db` | Chemin du fichier SQLite |
| `BACKEND_HOST` | `0.0.0.0` | Hôte du serveur |
| `BACKEND_PORT` | `8000` | Port du serveur |
| `FRONTEND_URL` | `http://localhost:4200` | URL du frontend (pour CORS) |
| `MIN_PROFIT_MARGIN` | `20` | Seuil minimum de profit (%) |
| `MAX_TRACKED_ITEMS` | `500` | Nombre max d'items analysés |
| `MAX_BUDGET_GOLD` | `500000` | Budget max en gold pour les suggestions |
| `SCAN_INTERVAL_MINUTES` | `60` | Intervalle entre les scans automatiques (minutes) |

### URLs Blizzard (calculées automatiquement)
| Méthode | URL |
|---|---|
| `BlizzardTokenURL()` | `https://oauth.battle.net/token` |
| `BlizzardAPIBaseURL()` | `https://{region}.api.blizzard.com` |
| `BlizzardAHURL()` | `.../{connected_realm_id}/auctions` (namespace: dynamic-{region}) |
| `BlizzardCommoditiesURL()` | `.../auctions/commodities` (namespace: dynamic-{region}) |
| Item details | `.../data/wow/item/{id}` (namespace: static-{region}) |
| Item media | `.../data/wow/media/item/{id}` (namespace: static-{region}) |

---

## 🔗 Dépendances Go (`go.mod`)

| Package | Version | Rôle |
|---|---|---|
| `github.com/gin-gonic/gin` | v1.10.0 | Framework HTTP |
| `github.com/gin-contrib/cors` | v1.7.2 | Middleware CORS |
| `gorm.io/gorm` | v1.25.12 | ORM |
| `gorm.io/driver/sqlite` | v1.5.6 | Driver SQLite (CGO) |
| `github.com/bwmarrin/discordgo` | v0.28.1 | Bot Discord |
| `github.com/go-co-op/gocron` | v1.37.0 | Scheduler cron |
| `github.com/joho/godotenv` | v1.5.1 | Chargement .env |

---

## 🚀 Commandes

```bash
# Build
cd backend-go
go build -o wow-ah-bot.exe ./cmd/server/

# Run (après avoir configuré .env)
./wow-ah-bot.exe

# Frontend dev
cd frontend
npm install
ng serve --proxy-config proxy.conf.json

# Accès
# API:       http://localhost:8000/api/dashboard
# Frontend:  http://localhost:4200
```

---

## 📝 Conventions

- **Monnaie** : tous les prix sont stockés en **copper** (1 gold = 10000 copper). Conversion uniquement dans les DTOs pour l'affichage.
- **Noms d'items** : toujours le vrai nom Blizzard localisé (`fr_FR`). Le fallback `"Item #ID"` ne doit jamais apparaître en production grâce aux 3 niveaux de résolution.
- **Imports circulaires** : évités via le pattern callback (`api.NotifyFunc` injecté par `main.go`).
- **Concurrence** : le scanner utilise des goroutines avec `sync.WaitGroup` pour le fetch d'items. Le bot Discord utilise un channel `ready` pour la synchronisation.
- **Batch inserts** : toutes les insertions massives (entries, price_history) sont en batch de 500-1000 pour ne pas surcharger SQLite.
- **SQLite** : mode WAL + `busy_timeout=30000ms` pour supporter les écritures concurrentes (scheduler + API).
