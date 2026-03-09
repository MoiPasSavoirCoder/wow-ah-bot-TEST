import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute } from '@angular/router';
import { Subject, takeUntil, switchMap, interval } from 'rxjs';

// PrimeNG
import { CardModule } from 'primeng/card';
import { ChartModule } from 'primeng/chart';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { DropdownModule } from 'primeng/dropdown';
import { FormsModule } from '@angular/forms';

import { ApiService } from '../../services/api.service';
import { PriceHistoryList, PriceHistoryEntry } from '../../models/interfaces';

@Component({
  selector: 'app-price-history',
  standalone: true,
  imports: [
    CommonModule, FormsModule,
    CardModule, ChartModule, TableModule, ButtonModule, DropdownModule,
  ],
  template: `
    <div class="price-history-page">
      <!-- Header -->
      <div class="page-header">
        <div>
          <h1>📈 Historique des Prix</h1>
          <p class="subtitle" *ngIf="priceData">
            {{ priceData.item_name || 'Item #' + itemId }}
          </p>
        </div>
        <div class="flex gap-2 align-items-center">
          <p-dropdown
            [options]="periodOptions"
            [(ngModel)]="selectedDays"
            (onChange)="loadPriceHistory()"
            styleClass="period-dropdown"
          ></p-dropdown>
        </div>
      </div>

      <!-- Price Chart -->
      <p-card header="Évolution des prix" class="mb-4">
        <p-chart
          *ngIf="chartData"
          type="line"
          [data]="chartData"
          [options]="chartOptions"
          height="400px"
        ></p-chart>
        <div *ngIf="!chartData" class="chart-placeholder">
          <i class="pi pi-chart-line" style="font-size: 3rem; opacity: 0.3;"></i>
          <p>Pas de données de prix disponibles</p>
        </div>
      </p-card>

      <!-- Price Data Table -->
      <p-card header="Données détaillées" *ngIf="priceData?.history?.length">
        <p-table
          [value]="priceData!.history"
          [rows]="20"
          [paginator]="true"
          [responsive]="true"
          styleClass="p-datatable-sm"
          sortField="scanned_at"
          [sortOrder]="-1"
        >
          <ng-template pTemplate="header">
            <tr>
              <th pSortableColumn="scanned_at">Date du scan</th>
              <th pSortableColumn="min_buyout">Prix min</th>
              <th pSortableColumn="avg_buyout">Prix moyen</th>
              <th pSortableColumn="median_buyout">Prix médian</th>
              <th pSortableColumn="total_quantity">Quantité totale</th>
              <th pSortableColumn="num_auctions">Nb enchères</th>
            </tr>
          </ng-template>
          <ng-template pTemplate="body" let-entry>
            <tr>
              <td>{{ entry.scanned_at | date:'dd/MM/yy HH:mm' }}</td>
              <td class="gold-amount font-bold">{{ entry.min_buyout_gold }}</td>
              <td class="gold-amount">{{ entry.avg_buyout_gold }}</td>
              <td>{{ formatCopper(entry.median_buyout) }}</td>
              <td>{{ entry.total_quantity | number }}</td>
              <td>{{ entry.num_auctions }}</td>
            </tr>
          </ng-template>
        </p-table>
      </p-card>
    </div>
  `,
  styles: [`
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 1.5rem;

      h1 { font-size: 1.75rem; font-weight: 800; }
      .subtitle { color: var(--accent-blue); font-size: 1rem; margin-top: 0.25rem; font-weight: 600; }
    }

    .chart-placeholder {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 350px;
      color: var(--text-color-secondary);
      gap: 1rem;
      i { font-size: 3rem; opacity: 0.2; }
      p { opacity: 0.5; font-size: 0.9rem; }
    }

    :host ::ng-deep .period-dropdown {
      min-width: 150px;
    }
  `],
})
export class PriceHistoryComponent implements OnInit, OnDestroy {
  itemId = 0;
  priceData: PriceHistoryList | null = null;
  chartData: any = null;
  chartOptions: any = null;
  selectedDays = 7;

  periodOptions = [
    { label: '24 heures', value: 1 },
    { label: '3 jours', value: 3 },
    { label: '7 jours', value: 7 },
    { label: '14 jours', value: 14 },
    { label: '30 jours', value: 30 },
    { label: '90 jours', value: 90 },
  ];

  private destroy$ = new Subject<void>();

  constructor(
    private route: ActivatedRoute,
    private api: ApiService,
  ) {}

  ngOnInit(): void {
    this.setupChartOptions();
    // Auto-refresh every 60 seconds
    interval(60_000)
      .pipe(takeUntil(this.destroy$))
      .subscribe(() => {
        if (this.itemId) this.loadPriceHistory();
      });

    this.route.params
      .pipe(
        takeUntil(this.destroy$),
        switchMap((params) => {
          this.itemId = +params['itemId'];
          return this.api.getPriceHistory(this.itemId, this.selectedDays);
        }),
      )
      .subscribe({
        next: (data) => {
          this.priceData = data;
          this.buildChart(data);
        },
        error: (err) => console.error('Price history error:', err),
      });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadPriceHistory(): void {
    this.api.getPriceHistory(this.itemId, this.selectedDays)
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: (data) => {
          this.priceData = data;
          this.buildChart(data);
        },
      });
  }

  formatCopper(copper: number | null): string {
    if (!copper) return '0g';
    const gold = Math.floor(copper / 10000);
    const silver = Math.floor((copper % 10000) / 100);
    return gold ? `${gold.toLocaleString('fr-FR')}g ${silver}s` : `${silver}s`;
  }

  private buildChart(data: PriceHistoryList): void {
    if (!data.history || data.history.length === 0) {
      this.chartData = null;
      return;
    }

    const labels = data.history.map((h) =>
      new Date(h.scanned_at).toLocaleDateString('fr-FR', {
        day: '2-digit',
        month: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })
    );

    this.chartData = {
      labels,
      datasets: [
        {
          label: 'Prix minimum',
          data: data.history.map((h) => h.min_buyout ? h.min_buyout / 10000 : null),
          borderColor: '#059669',
          backgroundColor: 'rgba(5,150,105,0.06)',
          fill: false,
          tension: 0.4,
          pointRadius: 3,
          pointBackgroundColor: '#059669',
          pointBorderColor: '#ffffff',
          pointBorderWidth: 2,
        },
        {
          label: 'Prix moyen',
          data: data.history.map((h) => h.avg_buyout ? h.avg_buyout / 10000 : null),
          borderColor: '#d97706',
          backgroundColor: 'rgba(217,119,6,0.06)',
          fill: false,
          tension: 0.4,
          pointRadius: 3,
          pointBackgroundColor: '#d97706',
          pointBorderColor: '#ffffff',
          pointBorderWidth: 2,
        },
        {
          label: 'Prix médian',
          data: data.history.map((h) => h.median_buyout ? h.median_buyout / 10000 : null),
          borderColor: '#2563eb',
          backgroundColor: 'rgba(37,99,235,0.06)',
          fill: true,
          tension: 0.4,
          pointRadius: 2,
          pointBackgroundColor: '#2563eb',
          pointBorderColor: '#ffffff',
          pointBorderWidth: 2,
        },
      ],
    };
  }

  private setupChartOptions(): void {
    this.chartOptions = {
      responsive: true,
      maintainAspectRatio: false,
      interaction: {
        mode: 'index',
        intersect: false,
      },
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
          callbacks: {
            label: (ctx: any) => ` ${ctx.dataset.label}: ${ctx.parsed.y?.toLocaleString('fr-FR')}g`,
          },
        },
      },
      scales: {
        x: {
          ticks: { color: '#64748b', maxRotation: 45, font: { family: 'Inter' } },
          grid: { color: '#f1f5f9' },
          border: { color: '#e2e8f0' },
        },
        y: {
          ticks: {
            color: '#64748b',
            font: { family: 'Inter' },
            callback: (value: number) => `${value.toLocaleString('fr-FR')}g`,
          },
          grid: { color: '#f1f5f9' },
          border: { color: '#e2e8f0' },
        },
      },
    };
  }
}
