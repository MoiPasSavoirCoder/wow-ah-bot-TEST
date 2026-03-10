import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { ApiService } from '../../services/api.service';
import { AIStats, AITrade, AISnapshot } from '../../models/interfaces';
import { GoldFormatPipe } from '../../pipes/gold-format.pipe';
import { interval, Subject, takeUntil } from 'rxjs';

// PrimeNG
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { TooltipModule } from 'primeng/tooltip';
import { ProgressBarModule } from 'primeng/progressbar';
import { TabViewModule } from 'primeng/tabview';
import { ChartModule } from 'primeng/chart';
import { SkeletonModule } from 'primeng/skeleton';
import { BadgeModule } from 'primeng/badge';

@Component({
  selector: 'app-ai-trading',
  standalone: true,
  imports: [
    CommonModule, DatePipe, GoldFormatPipe,
    TableModule, TagModule, TooltipModule, ProgressBarModule,
    TabViewModule, ChartModule, SkeletonModule, BadgeModule,
  ],
  template: `
    <!-- ═══ Page Header ═══ -->
    <div class="page-header">
      <div>
        <h1 class="page-title">
          <span class="title-icon">🤖</span>
          AI Trading Simulator
        </h1>
        <p class="page-subtitle">
          Simulation virtuelle d'achat-revente basée sur l'analyse IR — Budget initial : 100 000g
        </p>
      </div>
    </div>

    <!-- ═══ Stats Cards ═══ -->
    <div class="stats-grid" *ngIf="stats">
      <!-- Budget -->
      <div class="stat-card gold-card">
        <div class="stat-icon">💰</div>
        <div class="stat-content">
          <div class="stat-label">Valeur totale</div>
          <div class="stat-value gold-value">{{ stats.total_value_gold }}</div>
          <div class="stat-sub" [class.positive]="stats.roi_pct > 0" [class.negative]="stats.roi_pct < 0">
            ROI: {{ stats.roi_pct > 0 ? '+' : '' }}{{ stats.roi_pct }}%
          </div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon">🏦</div>
        <div class="stat-content">
          <div class="stat-label">Cash disponible</div>
          <div class="stat-value">{{ stats.current_cash_gold }}</div>
          <div class="stat-sub">Investi: {{ stats.invested_gold }}</div>
        </div>
      </div>

      <div class="stat-card" [class.positive-card]="stats.total_pnl_copper > 0" [class.negative-card]="stats.total_pnl_copper < 0">
        <div class="stat-icon">{{ stats.total_pnl_copper >= 0 ? '📈' : '📉' }}</div>
        <div class="stat-content">
          <div class="stat-label">P&L Total</div>
          <div class="stat-value" [class.positive]="stats.total_pnl_copper > 0" [class.negative]="stats.total_pnl_copper < 0">
            {{ stats.total_pnl_copper > 0 ? '+' : '' }}{{ stats.total_pnl_gold }}
          </div>
          <div class="stat-sub">
            Réalisé: {{ stats.realized_pnl_gold }} · Latent: {{ stats.unrealized_pnl_gold }}
          </div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon">🎯</div>
        <div class="stat-content">
          <div class="stat-label">Win Rate</div>
          <div class="stat-value">{{ stats.win_rate }}%</div>
          <div class="stat-sub">{{ stats.winning_trades }}W / {{ stats.losing_trades }}L</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon">📊</div>
        <div class="stat-content">
          <div class="stat-label">Trades</div>
          <div class="stat-value">{{ stats.total_trades }}</div>
          <div class="stat-sub">{{ stats.open_positions }} positions ouvertes</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon">⚡</div>
        <div class="stat-content">
          <div class="stat-label">Profit moyen</div>
          <div class="stat-value" [class.positive]="stats.avg_profit_pct > 0" [class.negative]="stats.avg_profit_pct < 0">
            {{ stats.avg_profit_pct > 0 ? '+' : '' }}{{ stats.avg_profit_pct }}%
          </div>
          <div class="stat-sub">
            Best: {{ stats.best_trade_gold }} · Worst: {{ stats.worst_trade_gold }}
          </div>
        </div>
      </div>
    </div>

    <!-- ═══ Loading Skeleton ═══ -->
    <div class="stats-grid" *ngIf="!stats">
      <div class="stat-card" *ngFor="let i of [1,2,3,4,5,6]">
        <p-skeleton width="100%" height="80px"></p-skeleton>
      </div>
    </div>

    <!-- ═══ P&L Chart ═══ -->
    <div class="chart-section" *ngIf="chartData">
      <h2 class="section-title">📈 Évolution du portefeuille</h2>
      <div class="chart-container">
        <p-chart type="line" [data]="chartData" [options]="chartOptions" height="350px"></p-chart>
      </div>
    </div>

    <!-- ═══ Tabs: Holdings / History ═══ -->
    <p-tabView styleClass="ai-tabs">

      <!-- ── Tab 1: Current Holdings ── -->
      <p-tabPanel>
        <ng-template pTemplate="header">
          <span>📦 Positions ouvertes</span>
          <p-badge *ngIf="holdings.length > 0" [value]="holdings.length.toString()" severity="info" class="ml-2"></p-badge>
        </ng-template>

        <p-table [value]="holdings" [paginator]="false" styleClass="p-datatable-sm ai-table"
                 [scrollable]="true" scrollHeight="500px" *ngIf="holdings.length > 0">
          <ng-template pTemplate="header">
            <tr>
              <th style="width: 40px"></th>
              <th>Item</th>
              <th style="text-align: center">Qté</th>
              <th style="text-align: right">Prix d'achat</th>
              <th style="text-align: right">Prix actuel</th>
              <th style="text-align: right">Objectif</th>
              <th style="text-align: right">Investi</th>
              <th style="text-align: right">P&L Latent</th>
              <th style="text-align: center">IR</th>
              <th style="text-align: center">Depuis</th>
            </tr>
          </ng-template>
          <ng-template pTemplate="body" let-h>
            <tr>
              <td>
                <img *ngIf="h.icon_url" [src]="h.icon_url" class="item-icon" [alt]="h.item_name">
              </td>
              <td>
                <span class="item-name">{{ h.item_name }}</span>
              </td>
              <td style="text-align: center">{{ h.quantity }}</td>
              <td style="text-align: right" class="gold-text">{{ h.price_per_unit_gold }}</td>
              <td style="text-align: right" class="gold-text">{{ h.current_price_gold }}</td>
              <td style="text-align: right" class="gold-text">{{ h.target_sell_gold }}</td>
              <td style="text-align: right" class="gold-text">{{ h.total_cost_gold }}</td>
              <td style="text-align: right"
                  [class.positive]="h.unrealized_pnl > 0"
                  [class.negative]="h.unrealized_pnl < 0">
                {{ h.unrealized_pnl > 0 ? '+' : '' }}{{ h.unrealized_pnl_gold }}
                <small>({{ h.unrealized_pnl_pct > 0 ? '+' : '' }}{{ h.unrealized_pnl_pct }}%)</small>
              </td>
              <td style="text-align: center">
                <span class="ir-badge" [class.ir-high]="h.rentability_index >= 60"
                      [class.ir-med]="h.rentability_index >= 45 && h.rentability_index < 60"
                      [class.ir-low]="h.rentability_index < 45">
                  {{ h.rentability_index | number:'1.0-0' }}
                </span>
              </td>
              <td style="text-align: center; font-size: 0.8rem; color: var(--text-color-secondary)">
                {{ h.created_at | date:'dd/MM HH:mm' }}
              </td>
            </tr>
          </ng-template>
        </p-table>

        <div class="empty-state" *ngIf="holdings.length === 0">
          <span class="empty-icon">📭</span>
          <p>Aucune position ouverte — l'IA attend les prochaines opportunités</p>
        </div>
      </p-tabPanel>

      <!-- ── Tab 2: Trade History ── -->
      <p-tabPanel>
        <ng-template pTemplate="header">
          <span>📜 Historique des trades</span>
          <p-badge *ngIf="trades.length > 0" [value]="trades.length.toString()" severity="secondary" class="ml-2"></p-badge>
        </ng-template>

        <p-table [value]="trades" [paginator]="true" [rows]="20" styleClass="p-datatable-sm ai-table"
                 [scrollable]="true" scrollHeight="500px" *ngIf="trades.length > 0">
          <ng-template pTemplate="header">
            <tr>
              <th style="width: 40px"></th>
              <th>Item</th>
              <th style="text-align: center">Status</th>
              <th style="text-align: center">Qté</th>
              <th style="text-align: right">Achat</th>
              <th style="text-align: right">Vente</th>
              <th style="text-align: right">Profit</th>
              <th style="text-align: center">Raison</th>
              <th style="text-align: center">IR</th>
              <th style="text-align: center">Date</th>
            </tr>
          </ng-template>
          <ng-template pTemplate="body" let-t>
            <tr [class.winning-row]="t.profit_copper > 0" [class.losing-row]="t.profit_copper < 0">
              <td>
                <img *ngIf="t.icon_url" [src]="t.icon_url" class="item-icon" [alt]="t.item_name">
              </td>
              <td>
                <span class="item-name">{{ t.item_name }}</span>
              </td>
              <td style="text-align: center">
                <p-tag [value]="t.status"
                       [severity]="t.status === 'SOLD' ? (t.profit_copper >= 0 ? 'success' : 'danger') : 'warning'">
                </p-tag>
              </td>
              <td style="text-align: center">{{ t.quantity }}</td>
              <td style="text-align: right" class="gold-text">{{ t.price_per_unit_gold }}</td>
              <td style="text-align: right" class="gold-text">{{ t.sell_price_gold || '—' }}</td>
              <td style="text-align: right"
                  [class.positive]="t.profit_copper > 0"
                  [class.negative]="t.profit_copper < 0">
                {{ t.profit_copper > 0 ? '+' : '' }}{{ t.profit_gold }}
                <small *ngIf="t.profit_pct">({{ t.profit_pct > 0 ? '+' : '' }}{{ t.profit_pct }}%)</small>
              </td>
              <td style="text-align: center">
                <p-tag *ngIf="t.sell_reason" [value]="reasonLabel(t.sell_reason)"
                       [severity]="reasonSeverity(t.sell_reason)" [rounded]="true">
                </p-tag>
              </td>
              <td style="text-align: center">
                <span class="ir-badge" [class.ir-high]="t.rentability_index >= 60"
                      [class.ir-med]="t.rentability_index >= 45 && t.rentability_index < 60">
                  {{ t.rentability_index | number:'1.0-0' }}
                </span>
              </td>
              <td style="text-align: center; font-size: 0.8rem; color: var(--text-color-secondary)">
                {{ t.created_at | date:'dd/MM HH:mm' }}
              </td>
            </tr>
          </ng-template>
        </p-table>

        <div class="empty-state" *ngIf="trades.length === 0">
          <span class="empty-icon">📜</span>
          <p>Aucun trade terminé pour le moment</p>
        </div>
      </p-tabPanel>
    </p-tabView>
  `,
  styles: [`
    :host { display: block; }

    .page-header {
      margin-bottom: 1.5rem;
    }
    .page-title {
      font-size: 1.8rem;
      font-weight: 700;
      color: #fff;
      margin: 0;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }
    .title-icon { font-size: 2rem; }
    .page-subtitle {
      color: var(--text-color-secondary, #aaa);
      margin: 0.25rem 0 0;
      font-size: 0.9rem;
    }

    /* ── Stats Grid ── */
    .stats-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 1rem;
      margin-bottom: 1.5rem;
    }
    .stat-card {
      background: var(--surface-card, #1e1e2e);
      border: 1px solid var(--surface-border, #333);
      border-radius: 12px;
      padding: 1.25rem;
      display: flex;
      gap: 1rem;
      align-items: flex-start;
      transition: transform 0.15s;
    }
    .stat-card:hover { transform: translateY(-2px); }
    .stat-icon { font-size: 2rem; }
    .stat-content { flex: 1; }
    .stat-label {
      font-size: 0.8rem;
      color: var(--text-color-secondary, #999);
      text-transform: uppercase;
      letter-spacing: 0.05em;
      margin-bottom: 0.25rem;
    }
    .stat-value {
      font-size: 1.4rem;
      font-weight: 700;
      color: #fff;
    }
    .stat-sub {
      font-size: 0.78rem;
      color: var(--text-color-secondary, #999);
      margin-top: 0.2rem;
    }

    .gold-card { border-color: #ffd100; }
    .gold-card .stat-value { color: #ffd100; }
    .positive-card { border-color: #4caf50; }
    .negative-card { border-color: #f44336; }

    .positive { color: #4caf50 !important; }
    .negative { color: #f44336 !important; }
    .gold-value { color: #ffd100 !important; }

    /* ── Chart ── */
    .chart-section {
      background: var(--surface-card, #1e1e2e);
      border: 1px solid var(--surface-border, #333);
      border-radius: 12px;
      padding: 1.25rem;
      margin-bottom: 1.5rem;
    }
    .section-title {
      font-size: 1.1rem;
      font-weight: 600;
      color: #fff;
      margin: 0 0 1rem;
    }
    .chart-container { width: 100%; }

    /* ── Tables ── */
    .gold-text { color: #ffd100; }
    .item-icon {
      width: 28px;
      height: 28px;
      border-radius: 4px;
      border: 1px solid #555;
    }
    .item-name {
      font-weight: 500;
      color: #e0e0e0;
    }

    .ir-badge {
      display: inline-block;
      padding: 2px 8px;
      border-radius: 10px;
      font-size: 0.8rem;
      font-weight: 600;
      background: #444;
      color: #ccc;
    }
    .ir-high { background: rgba(76, 175, 80, 0.2); color: #4caf50; }
    .ir-med  { background: rgba(255, 152, 0, 0.2); color: #ff9800; }
    .ir-low  { background: rgba(244, 67, 54, 0.2); color: #f44336; }

    .winning-row { background: rgba(76, 175, 80, 0.05); }
    .losing-row  { background: rgba(244, 67, 54, 0.05); }

    /* ── Empty State ── */
    .empty-state {
      text-align: center;
      padding: 3rem 1rem;
      color: var(--text-color-secondary, #999);
    }
    .empty-icon { font-size: 3rem; display: block; margin-bottom: 0.5rem; }

    /* ── Tabs ── */
    :host ::ng-deep .ai-tabs .p-tabview-panels {
      background: var(--surface-card, #1e1e2e);
      border: 1px solid var(--surface-border, #333);
      border-top: none;
      border-radius: 0 0 12px 12px;
      padding: 0;
    }
    :host ::ng-deep .ai-tabs .p-tabview-nav {
      border-color: var(--surface-border, #333);
    }
    :host ::ng-deep .ai-table .p-datatable-thead > tr > th {
      background: var(--surface-ground, #161620);
      border-color: var(--surface-border, #333);
      color: var(--text-color-secondary, #999);
      font-size: 0.8rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    :host ::ng-deep .ai-table .p-datatable-tbody > tr > td {
      border-color: var(--surface-border, #222);
      padding: 0.6rem 0.75rem;
    }
  `]
})
export class AiTradingComponent implements OnInit, OnDestroy {
  stats: AIStats | null = null;
  holdings: AITrade[] = [];
  trades: AITrade[] = [];
  snapshots: AISnapshot[] = [];

  chartData: any = null;
  chartOptions: any = null;

  private destroy$ = new Subject<void>();

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.loadAll();

    // Auto-refresh every 2 minutes
    interval(120_000)
      .pipe(takeUntil(this.destroy$))
      .subscribe(() => this.loadAll());
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadAll(): void {
    this.api.getAIStats().subscribe(s => this.stats = s);
    this.api.getAIHoldings().subscribe(h => this.holdings = h);
    this.api.getAITrades().subscribe(t => this.trades = t.filter(tr => tr.status !== 'HOLDING'));
    this.api.getAISnapshots(30).subscribe(snaps => {
      this.snapshots = snaps;
      this.buildChart();
    });
  }

  buildChart(): void {
    if (!this.snapshots.length) return;

    // Down-sample if too many points (keep ~100 max)
    let data = this.snapshots;
    if (data.length > 100) {
      const step = Math.ceil(data.length / 100);
      data = data.filter((_, i) => i % step === 0 || i === data.length - 1);
    }

    const labels = data.map(s => {
      const d = new Date(s.recorded_at);
      return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' }) + ' ' +
             d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' });
    });

    const initialBudget = 1000000000; // 100k gold in copper

    this.chartData = {
      labels,
      datasets: [
        {
          label: 'Valeur totale',
          data: data.map(s => s.total_value_copper / 10000), // in gold
          borderColor: '#ffd100',
          backgroundColor: 'rgba(255, 209, 0, 0.1)',
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 2,
        },
        {
          label: 'Cash',
          data: data.map(s => s.cash_copper / 10000),
          borderColor: '#4fc3f7',
          backgroundColor: 'transparent',
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 1.5,
          borderDash: [5, 3],
        },
        {
          label: 'Budget initial',
          data: data.map(() => initialBudget / 10000),
          borderColor: 'rgba(255,255,255,0.2)',
          backgroundColor: 'transparent',
          pointRadius: 0,
          borderWidth: 1,
          borderDash: [10, 5],
        },
      ],
    };

    this.chartOptions = {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: { labels: { color: '#ccc', usePointStyle: true } },
        tooltip: {
          callbacks: {
            label: (ctx: any) => {
              const val = ctx.raw;
              const gold = Math.floor(val);
              const silver = Math.floor((val - gold) * 100);
              return `${ctx.dataset.label}: ${gold.toLocaleString()}g ${silver}s`;
            },
          },
        },
      },
      scales: {
        x: {
          ticks: { color: '#888', maxRotation: 45, maxTicksLimit: 15 },
          grid: { color: 'rgba(255,255,255,0.05)' },
        },
        y: {
          ticks: {
            color: '#888',
            callback: (v: number) => v.toLocaleString() + 'g',
          },
          grid: { color: 'rgba(255,255,255,0.05)' },
        },
      },
    };
  }

  reasonLabel(reason: string): string {
    const map: Record<string, string> = {
      'TARGET_PROFIT': '🎯 Objectif',
      'STOP_LOSS': '🛑 Stop-Loss',
      'ABOVE_MEDIAN': '📊 Médiane',
      'EXPIRED': '⏰ Expiré',
    };
    return map[reason] || reason;
  }

  reasonSeverity(reason: string): 'success' | 'danger' | 'warning' | 'info' {
    switch (reason) {
      case 'TARGET_PROFIT': return 'success';
      case 'ABOVE_MEDIAN': return 'success';
      case 'STOP_LOSS': return 'danger';
      case 'EXPIRED': return 'warning';
      default: return 'info';
    }
  }
}
