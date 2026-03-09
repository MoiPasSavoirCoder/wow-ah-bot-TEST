import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { Subject, takeUntil, interval } from 'rxjs';

// PrimeNG
import { CardModule } from 'primeng/card';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { ButtonModule } from 'primeng/button';
import { ChartModule } from 'primeng/chart';
import { DialogModule } from 'primeng/dialog';
import { InputNumberModule } from 'primeng/inputnumber';
import { InputTextModule } from 'primeng/inputtext';
import { DropdownModule } from 'primeng/dropdown';
import { ToastModule } from 'primeng/toast';
import { TabViewModule } from 'primeng/tabview';
import { MessageService } from 'primeng/api';

import { ApiService } from '../../services/api.service';
import { PortfolioEntry, InventoryItem, PnlSummary } from '../../models/interfaces';
import { GoldFormatPipe } from '../../pipes/gold-format.pipe';

@Component({
  selector: 'app-portfolio',
  standalone: true,
  imports: [
    CommonModule, RouterLink, FormsModule, GoldFormatPipe,
    CardModule, TableModule, TagModule, ButtonModule, ChartModule,
    DialogModule, InputNumberModule, InputTextModule, DropdownModule,
    ToastModule, TabViewModule,
  ],
  providers: [MessageService],
  template: `
    <div class="portfolio-page">
      <!-- Header -->
      <div class="page-header">
        <div>
          <h1>💰 Portfolio</h1>
          <p class="subtitle">Suivi de vos investissements et P&L</p>
        </div>
        <p-button
          icon="pi pi-plus"
          label="Ajouter une transaction"
          severity="success"
          [rounded]="true"
          (onClick)="showAddDialog = true"
        ></p-button>
      </div>

      <!-- P&L Summary Cards -->
      <div class="stats-grid mb-4" *ngIf="pnl">
        <div class="stat-card">
          <div class="flex justify-content-between align-items-start">
            <div>
              <div class="stat-value gold-amount">{{ pnl.total_invested }}</div>
              <div class="stat-label">Total investi</div>
            </div>
            <i class="pi pi-shopping-cart stat-icon" style="color: var(--accent-blue);"></i>
          </div>
        </div>

        <div class="stat-card">
          <div class="flex justify-content-between align-items-start">
            <div>
              <div class="stat-value gold-amount">{{ pnl.total_revenue }}</div>
              <div class="stat-label">Revenus de ventes</div>
            </div>
            <i class="pi pi-money-bill stat-icon" style="color: var(--accent-gold);"></i>
          </div>
        </div>

        <div class="stat-card">
          <div class="flex justify-content-between align-items-start">
            <div>
              <div class="stat-value">{{ pnl.ah_fees }}</div>
              <div class="stat-label">Frais AH (5%)</div>
            </div>
            <i class="pi pi-percentage stat-icon" style="color: var(--accent-red);"></i>
          </div>
        </div>

        <div class="stat-card">
          <div class="flex justify-content-between align-items-start">
            <div>
              <div class="stat-value" [class.profit-positive]="pnl.realized_profit_copper >= 0" [class.profit-negative]="pnl.realized_profit_copper < 0">
                {{ pnl.realized_profit }}
              </div>
              <div class="stat-label">Profit net réalisé</div>
            </div>
            <i class="pi pi-chart-line stat-icon" [style.color]="pnl.realized_profit_copper >= 0 ? 'var(--accent-green)' : 'var(--accent-red)'"></i>
          </div>
        </div>
      </div>

      <!-- Tabs -->
      <p-tabView>
        <!-- Inventory Tab -->
        <p-tabPanel header="📦 Inventaire">
          <p-table
            [value]="inventory"
            [responsive]="true"
            styleClass="p-datatable-sm"
          >
            <ng-template pTemplate="header">
              <tr>
                <th>Item</th>
                <th>Quantité en stock</th>
                <th>Prix d'achat moyen</th>
                <th>Total investi</th>
                <th>Actions</th>
              </tr>
            </ng-template>
            <ng-template pTemplate="body" let-item>
              <tr>
                <td>
                  <a [routerLink]="['/prices', item.item_id]" class="item-link font-semibold">
                    {{ item.item_name || 'Item #' + item.item_id }}
                  </a>
                </td>
                <td class="font-semibold">{{ item.quantity }}x</td>
                <td class="gold-amount">{{ item.avg_buy_price | goldFormat }}</td>
                <td class="gold-amount">{{ item.total_invested | goldFormat }}</td>
                <td>
                  <p-button
                    icon="pi pi-dollar"
                    label="Vendre"
                    [rounded]="true"
                    [text]="true"
                    severity="warning"
                    (onClick)="openSellDialog(item)"
                  ></p-button>
                </td>
              </tr>
            </ng-template>
            <ng-template pTemplate="emptymessage">
              <tr>
                <td colspan="5" class="text-center" style="padding: 2rem; opacity: 0.5;">
                  Aucun item en inventaire 📦
                </td>
              </tr>
            </ng-template>
          </p-table>
        </p-tabPanel>

        <!-- Transactions Tab -->
        <p-tabPanel header="📋 Historique">
          <p-table
            [value]="transactions"
            [rows]="20"
            [paginator]="true"
            [responsive]="true"
            styleClass="p-datatable-sm"
            sortField="created_at"
            [sortOrder]="-1"
          >
            <ng-template pTemplate="header">
              <tr>
                <th>Date</th>
                <th>Action</th>
                <th>Item</th>
                <th>Quantité</th>
                <th>Prix unitaire</th>
                <th>Total</th>
                <th>Notes</th>
              </tr>
            </ng-template>
            <ng-template pTemplate="body" let-tx>
              <tr>
                <td>{{ tx.created_at | date:'dd/MM/yy HH:mm' }}</td>
                <td>
                  <p-tag
                    [value]="tx.action"
                    [severity]="tx.action === 'BUY' ? 'success' : 'danger'"
                  ></p-tag>
                </td>
                <td>
                  <a [routerLink]="['/prices', tx.item_id]" class="item-link">
                    {{ tx.item_name || 'Item #' + tx.item_id }}
                  </a>
                </td>
                <td>{{ tx.quantity }}x</td>
                <td class="gold-amount">{{ tx.price_per_unit | goldFormat }}</td>
                <td class="gold-amount font-bold">{{ tx.total_price_gold }}</td>
                <td style="color: var(--text-secondary); font-size: 0.85rem;">{{ tx.notes }}</td>
              </tr>
            </ng-template>
            <ng-template pTemplate="emptymessage">
              <tr>
                <td colspan="7" class="text-center" style="padding: 2rem; opacity: 0.5;">
                  Aucune transaction enregistrée
                </td>
              </tr>
            </ng-template>
          </p-table>
        </p-tabPanel>
      </p-tabView>
    </div>

    <!-- Add Transaction Dialog -->
    <p-dialog
      header="Ajouter une transaction"
      [(visible)]="showAddDialog"
      [modal]="true"
      [style]="{ width: '450px' }"
    >
      <div class="flex flex-column gap-3 p-3">
        <div>
          <label class="block mb-1 font-semibold">Action</label>
          <p-dropdown
            [options]="actionOptions"
            [(ngModel)]="newTx.action"
            placeholder="Acheter ou Vendre"
            styleClass="w-full"
          ></p-dropdown>
        </div>
        <div>
          <label class="block mb-1 font-semibold">Item ID</label>
          <p-inputNumber [(ngModel)]="newTx.item_id" [useGrouping]="false" styleClass="w-full" />
        </div>
        <div>
          <label class="block mb-1 font-semibold">Nom de l'item</label>
          <input pInputText [(ngModel)]="newTx.item_name" class="w-full" placeholder="Ex: Minerai de Khaz'gorite" />
        </div>
        <div>
          <label class="block mb-1 font-semibold">Quantité</label>
          <p-inputNumber [(ngModel)]="newTx.quantity" [min]="1" styleClass="w-full" />
        </div>
        <div>
          <label class="block mb-1 font-semibold">Prix unitaire (en copper)</label>
          <p-inputNumber [(ngModel)]="newTx.price_per_unit" [useGrouping]="true" styleClass="w-full" />
          <small class="text-xs" style="color: var(--text-secondary);">
            1 gold = 10 000 copper. Ex: 5g 50s = 55 000
          </small>
        </div>
        <div>
          <label class="block mb-1 font-semibold">Notes</label>
          <input pInputText [(ngModel)]="newTx.notes" class="w-full" placeholder="Optionnel" />
        </div>
      </div>

      <ng-template pTemplate="footer">
        <p-button label="Annuler" [text]="true" (onClick)="showAddDialog = false" />
        <p-button label="Enregistrer" icon="pi pi-check" severity="success" (onClick)="addTransaction()" />
      </ng-template>
    </p-dialog>

    <p-toast></p-toast>
  `,
  styles: [`
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 1.5rem;

      h1 { font-size: 1.75rem; font-weight: 800; }
      .subtitle { color: var(--text-color-secondary); font-size: 0.85rem; margin-top: 0.25rem; }
    }

    .stats-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 1rem;
    }

    .item-link {
      color: var(--accent-blue);
      text-decoration: none;
      font-weight: 500;
      &:hover { color: #7dd3fc; text-decoration: underline; }
    }
  `],
})
export class PortfolioComponent implements OnInit, OnDestroy {
  transactions: PortfolioEntry[] = [];
  inventory: InventoryItem[] = [];
  pnl: PnlSummary | null = null;
  showAddDialog = false;

  newTx = {
    action: 'BUY',
    item_id: 0,
    item_name: '',
    quantity: 1,
    price_per_unit: 0,
    notes: '',
  };

  actionOptions = [
    { label: '🟢 Acheter (BUY)', value: 'BUY' },
    { label: '🔴 Vendre (SELL)', value: 'SELL' },
  ];

  private destroy$ = new Subject<void>();

  constructor(
    private api: ApiService,
    private messageService: MessageService,
  ) {}

  ngOnInit(): void {
    this.loadAll();
    // Auto-refresh every 60 seconds
    interval(60_000)
      .pipe(takeUntil(this.destroy$))
      .subscribe(() => this.loadAll());
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadAll(): void {
    this.api.getPortfolio().pipe(takeUntil(this.destroy$)).subscribe({
      next: (txs) => (this.transactions = txs),
    });
    this.api.getInventory().pipe(takeUntil(this.destroy$)).subscribe({
      next: (inv) => (this.inventory = inv),
    });
    this.api.getPnl().pipe(takeUntil(this.destroy$)).subscribe({
      next: (pnl) => (this.pnl = pnl),
    });
  }

  addTransaction(): void {
    if (!this.newTx.item_id || !this.newTx.quantity || !this.newTx.price_per_unit) {
      this.messageService.add({ severity: 'warn', summary: 'Champs requis', detail: 'Remplissez tous les champs' });
      return;
    }

    this.api.addPortfolioEntry({
      item_id: this.newTx.item_id,
      item_name: this.newTx.item_name || undefined,
      action: this.newTx.action,
      quantity: this.newTx.quantity,
      price_per_unit: this.newTx.price_per_unit,
      notes: this.newTx.notes || undefined,
    })
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: () => {
          this.messageService.add({
            severity: 'success',
            summary: 'Transaction ajoutée ✅',
            detail: `${this.newTx.action} ${this.newTx.quantity}x ${this.newTx.item_name}`,
          });
          this.showAddDialog = false;
          this.resetNewTx();
          this.loadAll();
        },
        error: () => {
          this.messageService.add({ severity: 'error', summary: 'Erreur', detail: 'Impossible d\'enregistrer' });
        },
      });
  }

  openSellDialog(item: InventoryItem): void {
    this.newTx = {
      action: 'SELL',
      item_id: item.item_id,
      item_name: item.item_name || '',
      quantity: item.quantity,
      price_per_unit: 0,
      notes: `Vente depuis inventaire`,
    };
    this.showAddDialog = true;
  }

  private resetNewTx(): void {
    this.newTx = { action: 'BUY', item_id: 0, item_name: '', quantity: 1, price_per_unit: 0, notes: '' };
  }
}
