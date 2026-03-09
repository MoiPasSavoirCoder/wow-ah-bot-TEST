import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { Subject, takeUntil, interval, forkJoin } from 'rxjs';

// PrimeNG
import { CardModule } from 'primeng/card';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { ButtonModule } from 'primeng/button';
import { DropdownModule } from 'primeng/dropdown';
import { SelectButtonModule } from 'primeng/selectbutton';
import { TooltipModule } from 'primeng/tooltip';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { ToastModule } from 'primeng/toast';
import { SkeletonModule } from 'primeng/skeleton';
import { ProgressBarModule } from 'primeng/progressbar';
import { ConfirmationService, MessageService } from 'primeng/api';
import { FormsModule } from '@angular/forms';

import { ApiService } from '../../services/api.service';
import { Deal, ItemScore } from '../../models/interfaces';

@Component({
  selector: 'app-deals',
  standalone: true,
  imports: [
    CommonModule, RouterLink, FormsModule,
    CardModule, TableModule, TagModule, ButtonModule,
    DropdownModule, SelectButtonModule, TooltipModule,
    ConfirmDialogModule, ToastModule, SkeletonModule, ProgressBarModule,
  ],
  providers: [ConfirmationService, MessageService],
  template: `
    <div class="deals-page">

      <!-- Header -->
      <div class="page-header">
        <div>
          <h1>OpportunitÃ©s de Trading</h1>
          <p class="subtitle">
            {{ activeTab === 'deals' ? 'Deals dÃ©tectÃ©s, triables par colonne' : 'Scoring IR de tous les items analysÃ©s' }}
            â€” mise Ã  jour auto toutes les 60s
          </p>
        </div>
        <div class="header-actions">
          <span *ngIf="lastRefresh" class="last-refresh-label">
            Dernier scan : {{ lastRefresh | date:'HH:mm:ss' }}
          </span>
          <p-button
            icon="pi pi-sync"
            label="Scanner + Analyser"
            [loading]="refreshing"
            (onClick)="runRefresh()"
            severity="info"
            [rounded]="true"
          />
        </div>
      </div>

      <!-- Tabs -->
      <div class="tab-bar">
        <button class="tab-btn" [class.active]="activeTab === 'deals'" (click)="activeTab = 'deals'">
          <i class="pi pi-chart-bar"></i> Deals
          <span class="tab-count" *ngIf="pendingDeals.length">{{ pendingDeals.length }}</span>
        </button>
        <button class="tab-btn" [class.active]="activeTab === 'scores'" (click)="activeTab = 'scores'">
          <i class="pi pi-star"></i> Item Scores
          <span class="tab-count">{{ scores.length }}</span>
        </button>
      </div>

      <!-- Stats bar -->
      <div class="stats-bar" *ngIf="!loading">
        <ng-container *ngIf="activeTab === 'deals'">
          <div class="stat-chip">
            <span class="stat-value">{{ deals.length }}</span>
            <span class="stat-label">deals total</span>
          </div>
          <div class="stat-chip good" *ngIf="bestDeal">
            <span class="stat-value">{{ bestDeal.rentability_index | number:'1.0-0' }}</span>
            <span class="stat-label">meilleur IR</span>
          </div>
          <div class="stat-chip good" *ngIf="bestDeal">
            <span class="stat-value">+{{ bestDeal.profit_margin | number:'1.0-0' }}%</span>
            <span class="stat-label">meilleure marge</span>
          </div>
          <div class="stat-chip" *ngIf="lastScanInfo">
            <span class="stat-value">{{ lastScanInfo.total_auctions | number }}</span>
            <span class="stat-label">enchÃ¨res analysÃ©es</span>
          </div>
        </ng-container>
        <ng-container *ngIf="activeTab === 'scores'">
          <div class="stat-chip">
            <span class="stat-value">{{ scores.length }}</span>
            <span class="stat-label">items scorÃ©s</span>
          </div>
          <div class="stat-chip good" *ngIf="topScore">
            <span class="stat-value">{{ topScore.rentability_index | number:'1.0-0' }}</span>
            <span class="stat-label">IR max</span>
          </div>
          <div class="stat-chip" *ngIf="topScore">
            <span class="stat-value">{{ topScore.item_name }}</span>
            <span class="stat-label">meilleur item</span>
          </div>
        </ng-container>
      </div>

      <!-- â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â• TAB: DEALS â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â• -->
      <ng-container *ngIf="activeTab === 'deals'">

        <!-- Status filter -->
        <div class="filter-bar">
          <p-selectButton
            [options]="statusOptions"
            [(ngModel)]="statusFilter"
            optionLabel="label"
            optionValue="value"
            (onChange)="applyFilters()"
          />
        </div>

        <!-- Skeleton -->
        <div *ngIf="loading" class="skeleton-table">
          <p-skeleton *ngFor="let i of [1,2,3,4,5]" height="48px" borderRadius="8px"></p-skeleton>
        </div>

        <!-- Table -->
        <p-table
          *ngIf="!loading"
          [value]="filteredDeals"
          [sortField]="'rentability_index'"
          [sortOrder]="-1"
          [paginator]="filteredDeals.length > 20"
          [rows]="20"
          [rowsPerPageOptions]="[20, 50, 100]"
          styleClass="p-datatable-sm p-datatable-gridlines deals-table"
          [scrollable]="true"
          scrollHeight="calc(100vh - 340px)"
        >
          <ng-template pTemplate="header">
            <tr>
              <th style="width:48px"></th>
              <th pSortableColumn="item_name" style="min-width:180px">
                Item <p-sortIcon field="item_name" />
              </th>
              <th pSortableColumn="rentability_index" style="width:120px; text-align:center">
                IR <p-sortIcon field="rentability_index" />
              </th>
              <th pSortableColumn="profit_margin" style="width:110px; text-align:right">
                Marge % <p-sortIcon field="profit_margin" />
              </th>
              <th pSortableColumn="current_price" style="width:130px; text-align:right">
                Prix actuel <p-sortIcon field="current_price" />
              </th>
              <th pSortableColumn="avg_price" style="width:120px; text-align:right">
                Moy. 7j <p-sortIcon field="avg_price" />
              </th>
              <th pSortableColumn="suggested_buy_price" style="width:130px; text-align:right">
                Achat conseillÃ© <p-sortIcon field="suggested_buy_price" />
              </th>
              <th pSortableColumn="suggested_sell_price" style="width:130px; text-align:right">
                Revente est. <p-sortIcon field="suggested_sell_price" />
              </th>
              <th pSortableColumn="suggested_quantity" style="width:80px; text-align:center">
                QtÃ© <p-sortIcon field="suggested_quantity" />
              </th>
              <th style="width:120px; text-align:right">Profit net</th>
              <th pSortableColumn="status" style="width:110px; text-align:center">
                Statut <p-sortIcon field="status" />
              </th>
              <th style="width:100px; text-align:center">Actions</th>
            </tr>
          </ng-template>

          <ng-template pTemplate="body" let-deal>
            <tr [class.row-executed]="deal.status === 'EXECUTED'" [class.row-skipped]="deal.status === 'SKIPPED'">
              <!-- Icon -->
              <td style="padding:4px 8px">
                <img *ngIf="deal.icon_url" [src]="deal.icon_url" [alt]="deal.item_name" class="row-icon" (error)="onImgError($event)" />
                <div *ngIf="!deal.icon_url" class="row-icon-placeholder"><i class="pi pi-box"></i></div>
              </td>
              <!-- Name -->
              <td>
                <a [routerLink]="['/prices', deal.item_id]" class="item-link">
                  {{ deal.item_name || ('Item #' + deal.item_id) }}
                </a>
              </td>
              <!-- IR -->
              <td style="text-align:center; padding: 4px 8px">
                <div class="ir-cell">
                  <span class="ir-value" [ngClass]="irClass(deal.rentability_index)">{{ deal.rentability_index | number:'1.0-0' }}</span>
                  <div class="ir-bar-track">
                    <div class="ir-bar-fill" [ngClass]="irClass(deal.rentability_index)" [style.width.%]="deal.rentability_index"></div>
                  </div>
                </div>
              </td>
              <!-- Marge -->
              <td style="text-align:right">
                <span class="margin-val" [class.positive]="deal.profit_margin > 0">
                  {{ deal.profit_margin > 0 ? '+' : '' }}{{ deal.profit_margin | number:'1.0-1' }}%
                </span>
              </td>
              <!-- Prix actuel -->
              <td style="text-align:right; color: var(--accent-blue); font-weight:600">{{ deal.current_price_gold }}</td>
              <!-- Moy 7j -->
              <td style="text-align:right; color: var(--text-color-secondary)">{{ deal.avg_price_gold }}</td>
              <!-- Achat -->
              <td style="text-align:right; font-weight:600">{{ deal.suggested_buy_price_gold }}</td>
              <!-- Revente -->
              <td style="text-align:right">{{ deal.suggested_sell_price_gold }}</td>
              <!-- QtÃ© -->
              <td style="text-align:center">
                <span class="qty-chip">x{{ deal.suggested_quantity }}</span>
              </td>
              <!-- Profit -->
              <td style="text-align:right">
                <span class="profit-val">{{ deal.potential_profit_gold }}</span>
              </td>
              <!-- Statut -->
              <td style="text-align:center">
                <span class="status-badge status-{{deal.status.toLowerCase()}}">{{ statusLabel(deal.status) }}</span>
              </td>
              <!-- Actions -->
              <td style="text-align:center">
                <div class="row-actions" *ngIf="deal.status === 'PENDING'">
                  <button class="btn-sm-execute" (click)="executeDeal(deal)" pTooltip="Acheter" tooltipPosition="top">
                    <i class="pi pi-check"></i>
                  </button>
                  <button class="btn-sm-skip" (click)="skipDeal(deal)" pTooltip="Ignorer" tooltipPosition="top">
                    <i class="pi pi-times"></i>
                  </button>
                </div>
                <span *ngIf="deal.status !== 'PENDING'" class="done-label">
                  <i class="pi" [ngClass]="deal.status === 'EXECUTED' ? 'pi-check-circle' : 'pi-ban'"></i>
                </span>
              </td>
            </tr>
          </ng-template>

          <ng-template pTemplate="emptymessage">
            <tr>
              <td colspan="12" class="empty-row">
                <i class="pi pi-search"></i> Aucun deal trouvÃ© â€” lancez un scan pour analyser les enchÃ¨res.
              </td>
            </tr>
          </ng-template>
        </p-table>
      </ng-container>

      <!-- â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â• TAB: SCORES â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â• -->
      <ng-container *ngIf="activeTab === 'scores'">

        <!-- Skeleton -->
        <div *ngIf="loading" class="skeleton-table">
          <p-skeleton *ngFor="let i of [1,2,3,4,5]" height="48px" borderRadius="8px"></p-skeleton>
        </div>

        <p-table
          *ngIf="!loading"
          [value]="scores"
          [sortField]="'rentability_index'"
          [sortOrder]="-1"
          [paginator]="scores.length > 25"
          [rows]="25"
          [rowsPerPageOptions]="[25, 50, 100]"
          styleClass="p-datatable-sm p-datatable-gridlines scores-table"
          [scrollable]="true"
          scrollHeight="calc(100vh - 310px)"
        >
          <ng-template pTemplate="header">
            <tr>
              <th style="width:48px"></th>
              <th pSortableColumn="item_name" style="min-width:180px">Item <p-sortIcon field="item_name" /></th>
              <th pSortableColumn="rentability_index" style="width:140px; text-align:center">
                IR global <p-sortIcon field="rentability_index" />
              </th>
              <th pSortableColumn="score_undervaluation" style="width:110px; text-align:center">
                Sous-val. <p-sortIcon field="score_undervaluation" />
                <span class="weight-hint">(35%)</span>
              </th>
              <th pSortableColumn="score_momentum" style="width:110px; text-align:center">
                Momentum <p-sortIcon field="score_momentum" />
                <span class="weight-hint">(20%)</span>
              </th>
              <th pSortableColumn="score_liquidity" style="width:110px; text-align:center">
                LiquiditÃ© <p-sortIcon field="score_liquidity" />
                <span class="weight-hint">(20%)</span>
              </th>
              <th pSortableColumn="score_stability" style="width:110px; text-align:center">
                StabilitÃ© <p-sortIcon field="score_stability" />
                <span class="weight-hint">(15%)</span>
              </th>
              <th pSortableColumn="score_net_profit" style="width:110px; text-align:center">
                Profit net <p-sortIcon field="score_net_profit" />
                <span class="weight-hint">(10%)</span>
              </th>
              <th pSortableColumn="current_min_price" style="width:120px; text-align:right">
                Prix min <p-sortIcon field="current_min_price" />
              </th>
              <th pSortableColumn="avg_daily_volume" style="width:110px; text-align:right">
                Vol./jour <p-sortIcon field="avg_daily_volume" />
              </th>
              <th pSortableColumn="data_points" style="width:80px; text-align:center">
                Points <p-sortIcon field="data_points" />
              </th>
            </tr>
          </ng-template>

          <ng-template pTemplate="body" let-s>
            <tr>
              <!-- Icon -->
              <td style="padding:4px 8px">
                <img *ngIf="s.icon_url" [src]="s.icon_url" [alt]="s.item_name" class="row-icon" (error)="onImgError($event)" />
                <div *ngIf="!s.icon_url" class="row-icon-placeholder"><i class="pi pi-box"></i></div>
              </td>
              <!-- Name -->
              <td>
                <a [routerLink]="['/prices', s.item_id]" class="item-link">{{ s.item_name }}</a>
              </td>
              <!-- IR global -->
              <td style="padding: 4px 12px">
                <div class="ir-cell">
                  <span class="ir-value ir-lg" [ngClass]="irClass(s.rentability_index)">{{ s.rentability_index | number:'1.0-0' }}</span>
                  <div class="ir-bar-track">
                    <div class="ir-bar-fill" [ngClass]="irClass(s.rentability_index)" [style.width.%]="s.rentability_index"></div>
                  </div>
                </div>
              </td>
              <!-- Composantes -->
              <td style="text-align:center"><span class="comp-badge" [ngClass]="irClass(s.score_undervaluation)">{{ s.score_undervaluation | number:'1.0-0' }}</span></td>
              <td style="text-align:center"><span class="comp-badge" [ngClass]="irClass(s.score_momentum)">{{ s.score_momentum | number:'1.0-0' }}</span></td>
              <td style="text-align:center"><span class="comp-badge" [ngClass]="irClass(s.score_liquidity)">{{ s.score_liquidity | number:'1.0-0' }}</span></td>
              <td style="text-align:center"><span class="comp-badge" [ngClass]="irClass(s.score_stability)">{{ s.score_stability | number:'1.0-0' }}</span></td>
              <td style="text-align:center"><span class="comp-badge" [ngClass]="irClass(s.score_net_profit)">{{ s.score_net_profit | number:'1.0-0' }}</span></td>
              <!-- Prix min -->
              <td style="text-align:right; font-weight:600; color: var(--accent-blue)">{{ s.current_min_price_gold }}</td>
              <!-- Volume -->
              <td style="text-align:right">{{ s.avg_daily_volume | number:'1.0-0' }}</td>
              <!-- Data points -->
              <td style="text-align:center; color: var(--text-color-secondary)">{{ s.data_points }}</td>
            </tr>
          </ng-template>

          <ng-template pTemplate="emptymessage">
            <tr>
              <td colspan="11" class="empty-row">
                <i class="pi pi-star"></i> Aucun score disponible â€” lancez une analyse.
              </td>
            </tr>
          </ng-template>
        </p-table>
      </ng-container>

    </div>

    <p-toast></p-toast>
    <p-confirmDialog></p-confirmDialog>
  `,
  styles: [`
    .deals-page { padding: 0; }

    /* â”€â”€â”€ Header â”€â”€â”€ */
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 1rem;
    }
    .page-header h1 { font-size: 1.6rem; font-weight: 800; margin: 0; }
    .page-header .subtitle { color: var(--text-color-secondary); font-size: 0.82rem; margin-top: 0.2rem; }
    .header-actions { display: flex; gap: 0.75rem; align-items: center; }
    .last-refresh-label { font-size: 0.78rem; color: var(--text-color-secondary); opacity: 0.7; }

    /* â”€â”€â”€ Tabs â”€â”€â”€ */
    .tab-bar {
      display: flex;
      gap: 0.25rem;
      border-bottom: 2px solid var(--surface-border);
      margin-bottom: 1rem;
    }
    .tab-btn {
      display: flex;
      align-items: center;
      gap: 0.4rem;
      padding: 0.55rem 1.1rem;
      border: none;
      background: transparent;
      cursor: pointer;
      font-size: 0.9rem;
      font-weight: 600;
      color: var(--text-color-secondary);
      border-bottom: 2px solid transparent;
      margin-bottom: -2px;
      transition: color 0.15s, border-color 0.15s;
    }
    .tab-btn:hover { color: var(--text-color); }
    .tab-btn.active { color: #2563eb; border-bottom-color: #2563eb; }
    .tab-count {
      background: #2563eb;
      color: white;
      border-radius: 99px;
      padding: 0.05rem 0.45rem;
      font-size: 0.7rem;
      font-weight: 700;
    }

    /* â”€â”€â”€ Stats bar â”€â”€â”€ */
    .stats-bar { display: flex; gap: 0.75rem; margin-bottom: 1rem; flex-wrap: wrap; }
    .stat-chip {
      background: var(--surface-card);
      border: 1px solid var(--surface-border);
      border-radius: 10px;
      padding: 0.45rem 0.9rem;
      display: flex; align-items: baseline; gap: 0.4rem;
    }
    .stat-chip.good { border-color: #059669; background: #f0fdf4; }
    .stat-value { font-size: 1.1rem; font-weight: 700; }
    .stat-chip.good .stat-value { color: #059669; }
    .stat-label { font-size: 0.75rem; color: var(--text-color-secondary); }

    /* â”€â”€â”€ Filter bar â”€â”€â”€ */
    .filter-bar { margin-bottom: 0.75rem; }

    /* â”€â”€â”€ Skeleton â”€â”€â”€ */
    .skeleton-table { display: flex; flex-direction: column; gap: 0.5rem; }

    /* â”€â”€â”€ Tables â”€â”€â”€ */
    :host ::ng-deep .deals-table .p-datatable-wrapper,
    :host ::ng-deep .scores-table .p-datatable-wrapper {
      border-radius: 10px;
      border: 1px solid var(--surface-border);
      overflow: hidden;
    }
    :host ::ng-deep .p-datatable.p-datatable-sm .p-datatable-thead > tr > th {
      background: var(--surface-ground);
      font-size: 0.8rem;
      font-weight: 700;
      padding: 0.55rem 0.75rem;
      white-space: nowrap;
    }
    :host ::ng-deep .p-datatable.p-datatable-sm .p-datatable-tbody > tr > td {
      padding: 0.45rem 0.75rem;
      font-size: 0.84rem;
    }
    :host ::ng-deep .p-datatable.p-datatable-sm .p-datatable-tbody > tr:hover > td {
      background: var(--surface-hover) !important;
    }

    .row-executed td { opacity: 0.5; }
    .row-skipped td  { opacity: 0.4; }

    /* â”€â”€â”€ Row elements â”€â”€â”€ */
    .row-icon { width: 36px; height: 36px; border-radius: 6px; border: 1px solid var(--surface-border); object-fit: cover; display: block; }
    .row-icon-placeholder {
      width: 36px; height: 36px; border-radius: 6px; background: var(--surface-ground);
      border: 1px solid var(--surface-border); display: flex; align-items: center; justify-content: center;
      color: var(--text-color-secondary); font-size: 1rem;
    }
    .item-link { color: #2563eb; text-decoration: none; font-weight: 600; font-size: 0.85rem; }
    .item-link:hover { text-decoration: underline; }

    /* â”€â”€â”€ IR cell â”€â”€â”€ */
    .ir-cell { display: flex; flex-direction: column; gap: 3px; align-items: center; }
    .ir-value { font-size: 0.9rem; font-weight: 800; line-height: 1; }
    .ir-value.ir-lg { font-size: 1rem; }
    .ir-bar-track { width: 100%; height: 4px; background: var(--surface-border); border-radius: 99px; overflow: hidden; }
    .ir-bar-fill { height: 100%; border-radius: 99px; transition: width 0.3s; }

    /* â”€â”€â”€ IR color classes â”€â”€â”€ */
    .high   { color: #059669; }
    .medium { color: #d97706; }
    .low    { color: #dc2626; }
    .ir-bar-fill.high   { background: linear-gradient(90deg, #059669, #34d399); }
    .ir-bar-fill.medium { background: linear-gradient(90deg, #d97706, #fbbf24); }
    .ir-bar-fill.low    { background: linear-gradient(90deg, #dc2626, #f87171); }

    /* â”€â”€â”€ Misc cells â”€â”€â”€ */
    .margin-val { font-weight: 700; color: var(--text-color-secondary); font-size: 0.85rem; }
    .margin-val.positive { color: #059669; }
    .profit-val { font-weight: 700; color: #059669; font-size: 0.88rem; }
    .qty-chip {
      display: inline-block;
      background: var(--surface-ground);
      border: 1px solid var(--surface-border);
      border-radius: 6px;
      padding: 0.1rem 0.4rem;
      font-size: 0.75rem;
      font-weight: 600;
    }

    .status-badge { font-size: 0.72rem; border-radius: 6px; padding: 0.15rem 0.5rem; font-weight: 700; white-space: nowrap; }
    .status-pending  { background: #fef3c7; color: #92400e; }
    .status-executed { background: #d1fae5; color: #065f46; }
    .status-skipped  { background: #f1f5f9; color: #64748b; }
    .status-expired  { background: #fee2e2; color: #991b1b; }

    .row-actions { display: flex; gap: 0.3rem; justify-content: center; }
    .btn-sm-execute, .btn-sm-skip {
      width: 30px; height: 30px; border-radius: 6px; border: none; cursor: pointer;
      display: flex; align-items: center; justify-content: center; font-size: 0.85rem;
      transition: opacity 0.15s, background 0.15s;
    }
    .btn-sm-execute { background: #059669; color: white; }
    .btn-sm-execute:hover { opacity: 0.8; }
    .btn-sm-skip { background: var(--surface-ground); border: 1px solid var(--surface-border); color: var(--text-color-secondary); }
    .btn-sm-skip:hover { background: #fee2e2; color: #dc2626; border-color: #fca5a5; }
    .done-label { font-size: 1.1rem; color: var(--text-color-secondary); }

    /* â”€â”€â”€ Scores composantes â”€â”€â”€ */
    .comp-badge {
      display: inline-block;
      min-width: 36px;
      text-align: center;
      font-size: 0.82rem;
      font-weight: 700;
      padding: 0.1rem 0.35rem;
      border-radius: 6px;
    }
    .comp-badge.high   { background: #d1fae5; color: #065f46; }
    .comp-badge.medium { background: #fef3c7; color: #92400e; }
    .comp-badge.low    { background: #fee2e2; color: #991b1b; }

    .weight-hint { font-size: 0.68rem; color: var(--text-color-secondary); font-weight: 400; display: block; }

    .empty-row { text-align: center; padding: 2.5rem 1rem !important; color: var(--text-color-secondary); font-size: 0.9rem; }
    .empty-row i { margin-right: 0.5rem; }

    :host ::ng-deep .p-selectbutton .p-button { font-size: 0.82rem; padding: 0.35rem 0.85rem; }
  `],
})
export class DealsComponent implements OnInit, OnDestroy {
  activeTab: 'deals' | 'scores' = 'deals';

  deals: Deal[] = [];
  filteredDeals: Deal[] = [];
  scores: ItemScore[] = [];

  loading = true;
  refreshing = false;
  lastRefresh: Date | null = null;
  lastScanInfo: { total_auctions: number; unique_items: number; deals_count: number } | null = null;

  statusFilter = 'PENDING';
  statusOptions = [
    { label: 'En attente', value: 'PENDING' },
    { label: 'Tous',       value: '' },
    { label: 'ExÃ©cutÃ©s',   value: 'EXECUTED' },
    { label: 'IgnorÃ©s',    value: 'SKIPPED' },
    { label: 'ExpirÃ©s',    value: 'EXPIRED' },
  ];

  private destroy$ = new Subject<void>();

  constructor(
    private api: ApiService,
    private messageService: MessageService,
    private confirmService: ConfirmationService,
  ) {}

  get pendingDeals(): Deal[] {
    return this.deals.filter(d => d.status === 'PENDING');
  }

  get bestDeal(): Deal | null {
    return [...this.pendingDeals].sort((a, b) => b.rentability_index - a.rentability_index)[0] ?? null;
  }

  get topScore(): ItemScore | null {
    return this.scores[0] ?? null;
  }

  ngOnInit(): void {
    this.loadAll();
    interval(60_000)
      .pipe(takeUntil(this.destroy$))
      .subscribe(() => this.runRefresh());
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadAll(): void {
    this.loading = true;
    forkJoin({
      deals: this.api.getDeals(undefined, undefined, 200),
      scores: this.api.getScores(200),
    })
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: ({ deals, scores }) => {
          this.deals = deals;
          this.scores = scores;
          this.loading = false;
          this.applyFilters();
        },
        error: () => { this.loading = false; },
      });
  }

  applyFilters(): void {
    if (!this.statusFilter) {
      this.filteredDeals = [...this.deals];
    } else {
      this.filteredDeals = this.deals.filter(d => d.status === this.statusFilter);
    }
  }

  runRefresh(): void {
    if (this.refreshing) return;
    this.refreshing = true;
    this.api.triggerRefresh()
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: (res) => {
          this.refreshing = false;
          this.lastRefresh = new Date();
          this.lastScanInfo = res;
          this.messageService.add({
            severity: 'success',
            summary: 'Scan terminÃ©',
            detail: `${res.deals_count} deal(s) dÃ©tectÃ©(s)`,
            life: 4000,
          });
          this.loadAll();
        },
        error: () => {
          this.refreshing = false;
          this.messageService.add({ severity: 'error', summary: 'Erreur', detail: 'Le scan a Ã©chouÃ©' });
        },
      });
  }

  executeDeal(deal: Deal): void {
    this.confirmService.confirm({
      message: `Acheter ${deal.suggested_quantity}x ${deal.item_name || ('Item #' + deal.item_id)} Ã  ${deal.suggested_buy_price_gold} ?\nProfit estimÃ© : ${deal.potential_profit_gold}`,
      header: "Confirmer l'achat",
      icon: 'pi pi-check-circle',
      acceptLabel: 'Acheter',
      rejectLabel: 'Annuler',
      accept: () => {
        this.api.executeDeal(deal.id)
          .pipe(takeUntil(this.destroy$))
          .subscribe({
            next: () => {
              this.messageService.add({
                severity: 'success',
                summary: 'Deal exÃ©cutÃ©',
                detail: `${deal.item_name || 'Item'} ajoutÃ© au portfolio`,
              });
              this.loadAll();
            },
          });
      },
    });
  }

  skipDeal(deal: Deal): void {
    this.api.skipDeal(deal.id)
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: () => {
          this.messageService.add({
            severity: 'info',
            summary: 'Deal ignorÃ©',
            detail: deal.item_name || `Item #${deal.item_id}`,
          });
          this.loadAll();
        },
      });
  }

  statusLabel(status: string): string {
    switch (status) {
      case 'PENDING':  return 'En attente';
      case 'EXECUTED': return 'AchetÃ©';
      case 'SKIPPED':  return 'IgnorÃ©';
      case 'EXPIRED':  return 'ExpirÃ©';
      default: return status;
    }
  }

  irClass(score: number): string {
    if (score >= 65) return 'high';
    if (score >= 35) return 'medium';
    return 'low';
  }

  onImgError(event: Event): void {
    (event.target as HTMLImageElement).style.display = 'none';
  }

  trackById(_: number, deal: Deal): number {
    return deal.id;
  }
}
