import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { CardModule } from 'primeng/card';
import { ChartModule } from 'primeng/chart';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { ButtonModule } from 'primeng/button';
import { SkeletonModule } from 'primeng/skeleton';
import { TooltipModule } from 'primeng/tooltip';
import { DividerModule } from 'primeng/divider';
import { ProgressBarModule } from 'primeng/progressbar';
import { ToastModule } from 'primeng/toast';
import { MessageService } from 'primeng/api';
import { interval } from 'rxjs';
import { ApiService } from '../../services/api.service';
import { DashboardSummary, Deal } from '../../models/interfaces';
import { Subject, takeUntil } from 'rxjs';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
    CommonModule, RouterLink,
    CardModule, ChartModule, TableModule, TagModule, ButtonModule,
    SkeletonModule, TooltipModule, DividerModule, ProgressBarModule,
    ToastModule,
  ],
  providers: [MessageService],
  template: `
    <div class="page-header">
      <div>
        <h1 class="page-title">
          <i class="pi pi-chart-bar" style="color: var(--primary-color); margin-right: 0.5rem;"></i>
          Dashboard
        </h1>
        <p class="page-subtitle" *ngIf="data?.last_scan">
          Dernier scan : {{ data?.last_scan | date:'dd/MM/yyyy HH:mm' }}
        </p>
        <p class="page-subtitle" *ngIf="!data?.last_scan">
          Aucun scan effectué — cliquez sur Scanner pour commencer
        </p>
      </div>
      <div class="flex gap-2">
        <p-button
          icon="pi pi-sync"
          label="Recharger"
          [text]="true"
          severity="secondary"
          (onClick)="loadDashboard()"
        ></p-button>
        <p-button
          icon="pi pi-search"
          label="Scanner l'AH"
          [loading]="scanning"
          (onClick)="triggerScan()"
          severity="warning"
          [raised]="true"
        ></p-button>
      </div>
    </div>

    <!-- KPI Cards -->
    <div class="kpi-grid" *ngIf="data; else loadingKpis">
      <div class="kpi-card">
        <div class="kpi-icon" style="background: rgba(217,119,6,0.1);">
          <i class="pi pi-wallet" style="color: var(--accent-gold);"></i>
        </div>
        <div class="kpi-body">
          <div class="kpi-value gold-amount">{{ data.current_balance_gold }}</div>
          <div class="kpi-label">Balance actuelle</div>
        </div>
      </div>

      <div class="kpi-card">
        <div class="kpi-icon" [style.background]="isProfitPositive ? 'rgba(5,150,105,0.1)' : 'rgba(220,38,38,0.1)'">
          <i class="pi pi-chart-line" [style.color]="isProfitPositive ? 'var(--accent-green)' : 'var(--accent-red)'"></i>
        </div>
        <div class="kpi-body">
          <div class="kpi-value" [class.profit-positive]="isProfitPositive" [class.profit-negative]="!isProfitPositive">
            {{ data.total_profit_gold }}
          </div>
          <div class="kpi-label">Profit réalisé</div>
        </div>
      </div>

      <div class="kpi-card">
        <div class="kpi-icon" style="background: rgba(37,99,235,0.1);">
          <i class="pi pi-shopping-bag" style="color: var(--accent-blue);"></i>
        </div>
        <div class="kpi-body">
          <div class="kpi-value" style="color: var(--accent-blue);">{{ data.total_invested_gold }}</div>
          <div class="kpi-label">En cours d'investissement</div>
        </div>
      </div>

      <div class="kpi-card">
        <div class="kpi-icon" style="background: rgba(124,58,237,0.1);">
          <i class="pi pi-bolt" style="color: var(--accent-purple);"></i>
        </div>
        <div class="kpi-body">
          <div class="kpi-value" style="color: var(--accent-purple); font-size: 2rem;">{{ data.active_deals }}</div>
          <div class="kpi-label">Deals actifs (PENDING)</div>
        </div>
      </div>
    </div>

    <ng-template #loadingKpis>
      <div class="kpi-grid">
        <div class="kpi-card" *ngFor="let _ of [1,2,3,4]">
          <p-skeleton width="100%" height="80px" borderRadius="12px" />
        </div>
      </div>
    </ng-template>

    <!-- Chart -->
    <p-card styleClass="mt-4">
      <ng-template pTemplate="header">
        <div class="flex justify-content-between align-items-center p-3 pb-0">
          <span class="font-semibold text-lg">
            <i class="pi pi-chart-line mr-2" style="color: var(--primary-color);"></i>
            Évolution Gold &amp; P&amp;L
          </span>
        </div>
      </ng-template>
      <p-chart
        *ngIf="goldChartData; else noChart"
        type="line"
        [data]="goldChartData"
        [options]="goldChartOptions"
        height="300px"
      ></p-chart>
      <ng-template #noChart>
        <div class="empty-chart">
          <i class="pi pi-chart-line"></i>
          <p>Les données apparaîtront après les premiers scans</p>
        </div>
      </ng-template>
    </p-card>

    <!-- Recent Deals -->
    <p-card styleClass="mt-4">
      <ng-template pTemplate="header">
        <div class="flex justify-content-between align-items-center p-3 pb-0">
          <span class="font-semibold text-lg">
            <i class="pi pi-bolt mr-2" style="color: #f59e0b;"></i>
            Deals récents
          </span>
          <a routerLink="/deals" class="item-link text-sm">Voir tout →</a>
        </div>
      </ng-template>

      <p-table
        [value]="data?.recent_deals || []"
        [rows]="8"
        styleClass="p-datatable-sm p-datatable-striped"
        [responsive]="true"
      >
        <ng-template pTemplate="header">
          <tr>
            <th style="width:70px">Type</th>
            <th>Item</th>
            <th>Prix</th>
            <th>Marge</th>
            <th>Confiance</th>
            <th>Qté</th>
            <th>Statut</th>
            <th style="width:90px">Actions</th>
          </tr>
        </ng-template>
        <ng-template pTemplate="body" let-deal>
          <tr>
            <td>
              <p-tag
                [value]="deal.deal_type"
                [severity]="deal.deal_type === 'BUY' ? 'success' : 'danger'"
              />
            </td>
            <td>
              <a [routerLink]="['/prices', deal.item_id]" class="item-link">
                {{ deal.item_name || ('Item #' + deal.item_id) }}
              </a>
            </td>
            <td class="gold-amount text-sm">{{ deal.current_price_gold }}</td>
            <td>
              <span class="font-bold text-sm" [class]="deal.profit_margin >= 0 ? 'profit-positive' : 'profit-negative'">
                {{ deal.profit_margin > 0 ? '+' : '' }}{{ deal.profit_margin | number:'1.1-1' }}%
              </span>
            </td>
            <td>
              <div class="confidence-bar">
                <div
                  class="confidence-fill"
                  [style.width.%]="deal.confidence_score"
                  [class.high]="deal.confidence_score >= 70"
                  [class.medium]="deal.confidence_score >= 40 && deal.confidence_score < 70"
                  [class.low]="deal.confidence_score < 40"
                ></div>
              </div>
              <span class="text-xs" [class]="getConfidenceClass(deal.confidence_score)">
                {{ deal.confidence_score | number:'1.0-0' }}
              </span>
            </td>
            <td class="text-sm">{{ deal.suggested_quantity }}x</td>
            <td>
              <p-tag
                [value]="deal.status"
                [severity]="getStatusSeverity(deal.status)"
              />
            </td>
            <td>
              <div class="flex gap-1" *ngIf="deal.status === 'PENDING'">
                <p-button
                  icon="pi pi-check"
                  [rounded]="true" [text]="true"
                  severity="success" size="small"
                  pTooltip="Exécuter"
                  (onClick)="executeDeal(deal)"
                />
                <p-button
                  icon="pi pi-times"
                  [rounded]="true" [text]="true"
                  severity="danger" size="small"
                  pTooltip="Ignorer"
                  (onClick)="skipDeal(deal)"
                />
              </div>
            </td>
          </tr>
        </ng-template>
        <ng-template pTemplate="emptymessage">
          <tr>
            <td colspan="8" class="text-center" style="padding: 3rem; opacity: 0.5;">
              <i class="pi pi-search" style="font-size: 2rem; display: block; margin-bottom: 0.5rem;"></i>
              Aucun deal détecté. Lancez un scan pour commencer !
            </td>
          </tr>
        </ng-template>
      </p-table>
    </p-card>

    <p-toast />
  `,
  styles: [`
    .kpi-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 1rem;
      margin-bottom: 0;
    }

    .kpi-card {
      background: var(--surface-card);
      border: 1px solid var(--surface-border);
      border-radius: 12px;
      padding: 1.35rem 1.5rem;
      display: flex;
      align-items: center;
      gap: 1.1rem;
      transition: transform 0.2s, box-shadow 0.2s, border-color 0.2s;
      cursor: default;
      box-shadow: 0 1px 8px rgba(0,0,0,0.06);

      &:hover {
        transform: translateY(-3px);
        box-shadow: 0 8px 24px rgba(0,0,0,0.1);
        border-color: #cbd5e1;
      }
    }

    .kpi-icon {
      width: 50px;
      height: 50px;
      border-radius: 12px;
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;
      i { font-size: 1.3rem; }
    }

    .kpi-body { flex: 1; min-width: 0; }

    .kpi-value {
      font-size: 1.45rem;
      font-weight: 800;
      line-height: 1.2;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .kpi-label {
      font-size: 0.72rem;
      color: var(--text-color-secondary);
      margin-top: 0.2rem;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      font-weight: 600;
    }

    .empty-chart {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 260px;
      color: var(--text-color-secondary);
      gap: 0.75rem;
      i { font-size: 2.5rem; opacity: 0.25; }
      p { font-size: 0.9rem; opacity: 0.6; }
    }

    .confidence-bar {
      width: 55px;
      height: 5px;
      background: #e2e8f0;
      border-radius: 99px;
      overflow: hidden;
      display: inline-block;
      vertical-align: middle;
      margin-right: 4px;
    }

    .confidence-fill {
      height: 100%;
      border-radius: 99px;
      &.high   { background: linear-gradient(90deg, #059669, #10b981); }
      &.medium { background: linear-gradient(90deg, #d97706, #f59e0b); }
      &.low    { background: linear-gradient(90deg, #dc2626, #f87171); }
    }
  `],
})
export class DashboardComponent implements OnInit, OnDestroy {
  data: DashboardSummary | null = null;
  scanning = false;
  isProfitPositive = true;
  goldChartData: any = null;
  goldChartOptions: any = null;
  private destroy$ = new Subject<void>();

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.loadDashboard();
    this.setupChartOptions();
    // Auto-refresh every 60 seconds: scan+analyze then reload dashboard
    interval(60_000)
      .pipe(takeUntil(this.destroy$))
      .subscribe(() => {
        this.api.triggerRefresh().pipe(takeUntil(this.destroy$))
          .subscribe({ next: () => this.loadDashboard(), error: () => this.loadDashboard() });
      });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadDashboard(): void {
    this.api.getDashboard()
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: (data) => {
          this.data = data;
          this.isProfitPositive = !data.total_profit_gold.startsWith('-');
          this.buildGoldChart(data);
        },
        error: (err) => console.error('Dashboard load error:', err),
      });
  }

  triggerScan(): void {
    this.scanning = true;
    this.api.triggerRefresh()
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: () => {
          this.scanning = false;
          this.loadDashboard();
        },
        error: () => {
          this.scanning = false;
          this.loadDashboard();
        },
      });
  }

  executeDeal(deal: Deal): void {
    this.api.executeDeal(deal.id)
      .pipe(takeUntil(this.destroy$))
      .subscribe(() => this.loadDashboard());
  }

  skipDeal(deal: Deal): void {
    this.api.skipDeal(deal.id)
      .pipe(takeUntil(this.destroy$))
      .subscribe(() => this.loadDashboard());
  }

  getConfidenceClass(score: number): string {
    if (score >= 70) return 'confidence-high';
    if (score >= 40) return 'confidence-medium';
    return 'confidence-low';
  }

  getStatusSeverity(status: string): 'success' | 'info' | 'warning' | 'danger' | undefined {
    switch (status) {
      case 'EXECUTED': return 'success';
      case 'PENDING': return 'warning';
      case 'EXPIRED': return 'danger';
      case 'SKIPPED': return 'info';
      default: return undefined;
    }
  }

  private setupChartOptions(): void {
    this.goldChartOptions = {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: {
          labels: { color: '#64748b', font: { family: 'Inter', weight: '500' } },
        },
        tooltip: {
          backgroundColor: '#ffffff',
          borderColor: '#e2e8f0',
          borderWidth: 1,
          titleColor: '#1e293b',
          bodyColor: '#64748b',
          boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
        },
      },
      scales: {
        x: {
          ticks: { color: '#64748b', font: { family: 'Inter' } },
          grid: { color: '#f1f5f9' },
          border: { color: '#e2e8f0' },
        },
        y: {
          ticks: {
            color: '#64748b',
            font: { family: 'Inter' },
            callback: (value: number) => (value / 10000).toLocaleString('fr-FR') + 'g',
          },
          grid: { color: '#f1f5f9' },
          border: { color: '#e2e8f0' },
        },
      },
    };
  }

  private buildGoldChart(data: DashboardSummary): void {
    if (!data.gold_history || data.gold_history.length === 0) {
      this.goldChartData = null;
      return;
    }

    const labels = data.gold_history.map(h =>
      new Date(h.recorded_at).toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
    );

    this.goldChartData = {
      labels,
      datasets: [
        {
          label: 'Profit (or)',
          data: data.gold_history.map(h => h.profit_copper),
          borderColor: '#059669',
          backgroundColor: 'rgba(5,150,105,0.08)',
          fill: true,
          tension: 0.4,
          pointRadius: 4,
          pointBackgroundColor: '#059669',
          pointBorderColor: '#ffffff',
          pointBorderWidth: 2,
        },
        {
          label: 'Investi (or)',
          data: data.gold_history.map(h => h.invested_copper),
          borderColor: '#2563eb',
          backgroundColor: 'rgba(37,99,235,0.06)',
          fill: true,
          tension: 0.4,
          pointRadius: 4,
          pointBackgroundColor: '#2563eb',
          pointBorderColor: '#ffffff',
          pointBorderWidth: 2,
        },
      ],
    };
  }
}
